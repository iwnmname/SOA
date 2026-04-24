package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "github.com/ClickHouse/clickhouse-go/v2"
)

type AggregateDelta struct {
	MetricDate          time.Time
	DAU                 int64
	AvgWatchTimeSeconds float64
	ViewsStarted        int64
	ViewsFinished       int64
	ConversionRate      float64
	RetentionD1         float64
	RetentionD7         float64
	CohortSize          int64
	RawEvents           int64
}

type TopMovie struct {
	MetricDate time.Time
	MovieID    string
	Views      int64
	Rank       int
}

type ClickHouseRepo struct {
	db *sql.DB
}

func NewClickHouseRepo(dsn string) (*ClickHouseRepo, error) {
	db, err := sql.Open("clickhouse", dsn)
	if err != nil {
		return nil, fmt.Errorf("open clickhouse: %w", err)
	}
	return &ClickHouseRepo{db: db}, nil
}

func (r *ClickHouseRepo) Close() error {
	if r.db == nil {
		return nil
	}
	return r.db.Close()
}

func (r *ClickHouseRepo) Ping(ctx context.Context) error {
	return r.db.PingContext(ctx)
}

func (r *ClickHouseRepo) LoadDailyMetrics(ctx context.Context, metricDate time.Time) (AggregateDelta, error) {
	const query = `
		WITH
			toDate(?) AS d,
			cohort AS (
				SELECT user_id
				FROM (
					SELECT user_id, minMerge(first_view_date_state) AS first_view_date
					FROM agg_user_first_view
					GROUP BY user_id
				)
				WHERE first_view_date = d
			),
			cohort_size AS (
				SELECT count() AS size FROM cohort
			),
			retention_d1_users AS (
				SELECT uniqExact(c.user_id) AS retained
				FROM cohort c
				INNER JOIN agg_daily_user_activity a ON a.user_id = c.user_id
				WHERE a.event_date = addDays(d, 1) AND a.view_started > 0
			),
			retention_d7_users AS (
				SELECT uniqExact(c.user_id) AS retained
				FROM cohort c
				INNER JOIN agg_daily_user_activity a ON a.user_id = c.user_id
				WHERE a.event_date = addDays(d, 7) AND a.view_started > 0
			)
		SELECT
			d AS metric_date,
			coalesce(uniqExact(user_id), 0) AS dau,
			coalesce(sum(finished_progress_sum) / nullIf(sum(view_finished), 0), 0) AS avg_watch_time_seconds,
			coalesce(sum(view_started), 0) AS views_started,
			coalesce(sum(view_finished), 0) AS views_finished,
			coalesce(sum(view_finished) / nullIf(sum(view_started), 0), 0) AS conversion_rate,
			(SELECT size FROM cohort_size) AS cohort_size,
			coalesce((SELECT retained FROM retention_d1_users) / nullIf((SELECT size FROM cohort_size), 0), 0) AS retention_d1,
			coalesce((SELECT retained FROM retention_d7_users) / nullIf((SELECT size FROM cohort_size), 0), 0) AS retention_d7
		FROM agg_daily_user_activity
		WHERE event_date = d
	`

	var delta AggregateDelta
	if err := r.db.QueryRowContext(ctx, query, metricDate).
		Scan(
			&delta.MetricDate,
			&delta.DAU,
			&delta.AvgWatchTimeSeconds,
			&delta.ViewsStarted,
			&delta.ViewsFinished,
			&delta.ConversionRate,
			&delta.CohortSize,
			&delta.RetentionD1,
			&delta.RetentionD7,
		); err != nil {
		return AggregateDelta{}, fmt.Errorf("load daily metrics from clickhouse: %w", err)
	}

	rawEvents, err := r.CountRawEventsForDate(ctx, metricDate)
	if err != nil {
		return AggregateDelta{}, err
	}
	delta.RawEvents = rawEvents

	return delta, nil
}

func (r *ClickHouseRepo) LoadTopMovies(ctx context.Context, metricDate time.Time, limit int) ([]TopMovie, error) {
	const query = `
		SELECT
			toDate(?) AS metric_date,
			movie_id,
			sum(view_started) AS views
		FROM agg_daily_movie_views
		WHERE event_date = toDate(?)
		GROUP BY movie_id
		ORDER BY views DESC, movie_id ASC
		LIMIT ?
	`

	rows, err := r.db.QueryContext(ctx, query, metricDate, metricDate, limit)
	if err != nil {
		return nil, fmt.Errorf("load top movies from clickhouse: %w", err)
	}
	defer rows.Close()

	result := make([]TopMovie, 0, limit)
	rank := 1
	for rows.Next() {
		var item TopMovie
		if err := rows.Scan(&item.MetricDate, &item.MovieID, &item.Views); err != nil {
			return nil, fmt.Errorf("scan top movie row: %w", err)
		}
		item.Rank = rank
		rank++
		result = append(result, item)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate top movies rows: %w", err)
	}

	return result, nil
}

func (r *ClickHouseRepo) CountRawEventsForDate(ctx context.Context, metricDate time.Time) (int64, error) {
	const query = `
		SELECT count()
		FROM movie_events
		WHERE event_date = toDate(?)
	`

	var count int64
	if err := r.db.QueryRowContext(ctx, query, metricDate).Scan(&count); err != nil {
		return 0, fmt.Errorf("count raw events for date: %w", err)
	}

	return count, nil
}
