package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"path"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"

	"github.com/online-cinema/aggregator/internal/repository"
)

var ErrAlreadyRunning = errors.New("aggregation cycle is already running")
var ErrExportAlreadyRunning = errors.New("export cycle is already running")

type Result struct {
	MetricDate         string        `json:"metric_date"`
	ProcessedRawEvents int64         `json:"processed_raw_events"`
	SavedMetrics       int           `json:"saved_metrics"`
	Duration           time.Duration `json:"duration"`
}

type ExportResult struct {
	MetricDate string        `json:"metric_date"`
	Rows       int           `json:"rows"`
	Bucket     string        `json:"bucket"`
	ObjectKey  string        `json:"object_key"`
	ETag       string        `json:"etag"`
	Duration   time.Duration `json:"duration"`
}

type Service struct {
	clickhouse *repository.ClickHouseRepo
	postgres   *repository.PostgresRepo
	s3         *repository.S3Repo
	logger     *zap.Logger
	exportCfg  ExportConfig

	aggMu      sync.Mutex
	aggRunning bool

	exportMu      sync.Mutex
	exportRunning bool
}

type ExportConfig struct {
	Interval   time.Duration
	Prefix     string
	MaxRetries int
}

func New(
	clickhouse *repository.ClickHouseRepo,
	postgres *repository.PostgresRepo,
	s3 *repository.S3Repo,
	logger *zap.Logger,
	exportCfg ExportConfig,
) *Service {
	if exportCfg.MaxRetries < 1 {
		exportCfg.MaxRetries = 3
	}
	if exportCfg.Prefix == "" {
		exportCfg.Prefix = "daily"
	}
	return &Service{
		clickhouse: clickhouse,
		postgres:   postgres,
		s3:         s3,
		logger:     logger,
		exportCfg:  exportCfg,
	}
}

func (s *Service) RunCycleForDate(ctx context.Context, metricDate time.Time) (Result, error) {
	s.aggMu.Lock()
	if s.aggRunning {
		s.aggMu.Unlock()
		return Result{}, ErrAlreadyRunning
	}
	s.aggRunning = true
	s.aggMu.Unlock()
	defer func() {
		s.aggMu.Lock()
		s.aggRunning = false
		s.aggMu.Unlock()
	}()

	dateUTC := time.Date(metricDate.UTC().Year(), metricDate.UTC().Month(), metricDate.UTC().Day(), 0, 0, 0, 0, time.UTC)

	started := time.Now()
	s.logger.Info("aggregation cycle started", zap.String("metric_date", dateUTC.Format("2006-01-02")))

	delta, err := s.clickhouse.LoadDailyMetrics(ctx, dateUTC)
	if err != nil {
		return Result{}, err
	}
	topMovies, err := s.clickhouse.LoadTopMovies(ctx, dateUTC, 10)
	if err != nil {
		return Result{}, err
	}

	metricRows := buildMetricRows(delta, topMovies)
	if err := s.persistWithRetry(ctx, metricRows); err != nil {
		return Result{}, err
	}

	result := Result{
		MetricDate:         dateUTC.Format("2006-01-02"),
		ProcessedRawEvents: delta.RawEvents,
		SavedMetrics:       len(metricRows),
		Duration:           time.Since(started),
	}

	s.logger.Info("aggregation cycle completed",
		zap.String("metric_date", result.MetricDate),
		zap.Int64("processed_raw_events", result.ProcessedRawEvents),
		zap.Int("saved_metrics", result.SavedMetrics),
		zap.Duration("duration", result.Duration),
	)

	return result, nil
}

func (s *Service) RunTicker(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	s.logger.Info("aggregation scheduler started", zap.Duration("interval", interval))
	for {
		select {
		case <-ctx.Done():
			s.logger.Info("aggregation scheduler stopped")
			return
		case <-ticker.C:
			cycleCtx, cancel := context.WithTimeout(ctx, interval)
			_, err := s.RunCycleForDate(cycleCtx, time.Now().UTC())
			cancel()
			if err != nil && !errors.Is(err, ErrAlreadyRunning) {
				s.logger.Error("scheduled aggregation failed", zap.Error(err))
			}
		}
	}
}

func (s *Service) RunExportForDate(ctx context.Context, metricDate time.Time) (ExportResult, error) {
	s.exportMu.Lock()
	if s.exportRunning {
		s.exportMu.Unlock()
		return ExportResult{}, ErrExportAlreadyRunning
	}
	s.exportRunning = true
	s.exportMu.Unlock()
	defer func() {
		s.exportMu.Lock()
		s.exportRunning = false
		s.exportMu.Unlock()
	}()

	dateUTC := time.Date(metricDate.UTC().Year(), metricDate.UTC().Month(), metricDate.UTC().Day(), 0, 0, 0, 0, time.UTC)
	started := time.Now()

	s.logger.Info("export cycle started", zap.String("metric_date", dateUTC.Format("2006-01-02")))

	metrics, err := s.loadMetricsWithRetry(ctx, dateUTC)
	if err != nil {
		return ExportResult{}, err
	}

	body, err := buildExportPayload(dateUTC, metrics)
	if err != nil {
		return ExportResult{}, err
	}

	objectKey := buildExportObjectKey(s.exportCfg.Prefix, dateUTC)
	etag, err := s.putObjectWithRetry(ctx, objectKey, body)
	if err != nil {
		return ExportResult{}, err
	}

	result := ExportResult{
		MetricDate: dateUTC.Format("2006-01-02"),
		Rows:       len(metrics),
		Bucket:     s.s3.Bucket(),
		ObjectKey:  objectKey,
		ETag:       etag,
		Duration:   time.Since(started),
	}

	s.logger.Info("export cycle completed",
		zap.String("metric_date", result.MetricDate),
		zap.Int("rows", result.Rows),
		zap.String("bucket", result.Bucket),
		zap.String("object_key", result.ObjectKey),
		zap.String("etag", result.ETag),
		zap.Duration("duration", result.Duration),
	)

	return result, nil
}

func (s *Service) RunExportTicker(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	s.logger.Info("export scheduler started", zap.Duration("interval", interval))
	for {
		select {
		case <-ctx.Done():
			s.logger.Info("export scheduler stopped")
			return
		case <-ticker.C:
			targetDate := time.Now().UTC().AddDate(0, 0, -1)
			cycleCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
			_, err := s.RunExportForDate(cycleCtx, targetDate)
			cancel()
			if err != nil && !errors.Is(err, ErrExportAlreadyRunning) {
				s.logger.Error("scheduled export failed", zap.Error(err))
			}
		}
	}
}

func buildMetricRows(delta repository.AggregateDelta, topMovies []repository.TopMovie) []repository.MetricRow {
	rows := []repository.MetricRow{
		{MetricDate: delta.MetricDate, MetricName: "dau", Value: float64(delta.DAU)},
		{MetricDate: delta.MetricDate, MetricName: "avg_watch_time_seconds", Value: delta.AvgWatchTimeSeconds},
		{MetricDate: delta.MetricDate, MetricName: "views_started", Value: float64(delta.ViewsStarted)},
		{MetricDate: delta.MetricDate, MetricName: "views_finished", Value: float64(delta.ViewsFinished)},
		{MetricDate: delta.MetricDate, MetricName: "conversion_rate", Value: delta.ConversionRate},
		{MetricDate: delta.MetricDate, MetricName: "retention_d1", Value: delta.RetentionD1},
		{MetricDate: delta.MetricDate, MetricName: "retention_d7", Value: delta.RetentionD7},
		{MetricDate: delta.MetricDate, MetricName: "retention_cohort_size", Value: float64(delta.CohortSize)},
	}

	for _, movie := range topMovies {
		rows = append(rows, repository.MetricRow{
			MetricDate: movie.MetricDate,
			MetricName: "top_movie_views",
			Dimension:  fmt.Sprintf("rank=%d;movie_id=%s", movie.Rank, movie.MovieID),
			Value:      float64(movie.Views),
		})
	}

	return rows
}

func (s *Service) persistWithRetry(ctx context.Context, rows []repository.MetricRow) error {
	const maxRetries = 4
	const baseBackoff = 200 * time.Millisecond
	const maxBackoff = 2 * time.Second

	var lastErr error
	for attempt := 1; attempt <= maxRetries; attempt++ {
		if err := s.postgres.UpsertMetrics(ctx, rows); err == nil {
			return nil
		} else {
			lastErr = err
		}

		if attempt == maxRetries {
			break
		}

		backoff := time.Duration(float64(baseBackoff) * math.Pow(2, float64(attempt-1)))
		if backoff > maxBackoff {
			backoff = maxBackoff
		}

		s.logger.Warn("postgres write failed, retrying",
			zap.Int("attempt", attempt),
			zap.Duration("backoff", backoff),
			zap.Error(lastErr),
		)

		timer := time.NewTimer(backoff)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}

	return fmt.Errorf("write metrics to postgres after retries: %w", lastErr)
}

func (s *Service) loadMetricsWithRetry(ctx context.Context, metricDate time.Time) ([]repository.BusinessMetric, error) {
	const baseBackoff = 200 * time.Millisecond
	const maxBackoff = 2 * time.Second

	var (
		lastErr error
		rows    []repository.BusinessMetric
	)

	for attempt := 1; attempt <= s.exportCfg.MaxRetries; attempt++ {
		rows, lastErr = s.postgres.LoadMetricsByDate(ctx, metricDate)
		if lastErr == nil {
			return rows, nil
		}

		if attempt == s.exportCfg.MaxRetries {
			break
		}

		backoff := exponentialBackoff(baseBackoff, maxBackoff, attempt)
		s.logger.Warn("load metrics from postgres failed, retrying",
			zap.Int("attempt", attempt),
			zap.Duration("backoff", backoff),
			zap.Error(lastErr),
		)

		if err := waitBackoff(ctx, backoff); err != nil {
			return nil, err
		}
	}

	return nil, fmt.Errorf("load metrics from postgres after retries: %w", lastErr)
}

func (s *Service) putObjectWithRetry(ctx context.Context, key string, body []byte) (string, error) {
	const baseBackoff = 200 * time.Millisecond
	const maxBackoff = 2 * time.Second

	var (
		lastErr error
		etag    string
	)

	for attempt := 1; attempt <= s.exportCfg.MaxRetries; attempt++ {
		tag, err := s.s3.PutObject(ctx, key, body, "application/json")
		if err == nil {
			etag = tag
			return etag, nil
		}
		lastErr = err

		if attempt == s.exportCfg.MaxRetries {
			break
		}

		backoff := exponentialBackoff(baseBackoff, maxBackoff, attempt)
		s.logger.Warn("put object to s3 failed, retrying",
			zap.Int("attempt", attempt),
			zap.Duration("backoff", backoff),
			zap.String("object_key", key),
			zap.Error(lastErr),
		)

		if err := waitBackoff(ctx, backoff); err != nil {
			return "", err
		}
	}

	return "", fmt.Errorf("put object to s3 after retries: %w", lastErr)
}

func buildExportObjectKey(prefix string, metricDate time.Time) string {
	cleanPrefix := strings.Trim(prefix, "/")
	if cleanPrefix == "" {
		cleanPrefix = "daily"
	}
	date := metricDate.UTC().Format("2006-01-02")
	return path.Join(cleanPrefix, date, "aggregates.json")
}

func buildExportPayload(metricDate time.Time, rows []repository.BusinessMetric) ([]byte, error) {
	type exportRow struct {
		MetricDate string  `json:"metric_date"`
		MetricName string  `json:"metric_name"`
		Dimension  string  `json:"dimension"`
		Value      float64 `json:"value"`
		ComputedAt string  `json:"computed_at"`
	}

	payload := make([]exportRow, 0, len(rows))
	for _, row := range rows {
		payload = append(payload, exportRow{
			MetricDate: metricDate.UTC().Format("2006-01-02"),
			MetricName: row.MetricName,
			Dimension:  row.Dimension,
			Value:      row.Value,
			ComputedAt: row.ComputedAt.UTC().Format(time.RFC3339),
		})
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal export payload: %w", err)
	}
	return body, nil
}

func exponentialBackoff(base, max time.Duration, attempt int) time.Duration {
	backoff := time.Duration(float64(base) * math.Pow(2, float64(attempt-1)))
	if backoff > max {
		return max
	}
	return backoff
}

func waitBackoff(ctx context.Context, backoff time.Duration) error {
	timer := time.NewTimer(backoff)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

