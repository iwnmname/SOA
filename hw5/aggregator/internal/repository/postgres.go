package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

type PostgresRepo struct {
	db *sql.DB
}

type MetricRow struct {
	MetricDate time.Time
	MetricName string
	Dimension  string
	Value      float64
}

type BusinessMetric struct {
	MetricDate time.Time
	MetricName string
	Dimension  string
	Value      float64
	ComputedAt time.Time
}

func NewPostgresRepo(dsn string) (*PostgresRepo, error) {
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, fmt.Errorf("open postgres: %w", err)
	}
	return &PostgresRepo{db: db}, nil
}

func (r *PostgresRepo) Close() error {
	if r.db == nil {
		return nil
	}
	return r.db.Close()
}

func (r *PostgresRepo) Ping(ctx context.Context) error {
	return r.db.PingContext(ctx)
}

func (r *PostgresRepo) EnsureSchema(ctx context.Context) error {
	statements := []string{
		`CREATE TABLE IF NOT EXISTS business_metrics (
			metric_date date NOT NULL,
			metric_name text NOT NULL,
			dimension text NOT NULL DEFAULT '',
			metric_value double precision NOT NULL,
			computed_at timestamptz NOT NULL DEFAULT now(),
			PRIMARY KEY (metric_date, metric_name, dimension)
		)`,
	}

	for _, statement := range statements {
		if _, err := r.db.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("apply schema statement: %w", err)
		}
	}
	return nil
}

func (r *PostgresRepo) UpsertMetrics(ctx context.Context, rows []MetricRow) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	const upsertMetric = `
		INSERT INTO business_metrics (
			metric_date,
			metric_name,
			dimension,
			metric_value,
			computed_at
		) VALUES ($1, $2, $3, $4, now())
		ON CONFLICT (metric_date, metric_name, dimension)
		DO UPDATE SET
			metric_value = EXCLUDED.metric_value,
			computed_at = now()
	`

	for _, row := range rows {
		if _, err := tx.ExecContext(ctx, upsertMetric,
			row.MetricDate,
			row.MetricName,
			row.Dimension,
			row.Value,
		); err != nil {
			return fmt.Errorf("upsert business metric row: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}
	return nil
}

func (r *PostgresRepo) LoadMetricsByDate(ctx context.Context, metricDate time.Time) ([]BusinessMetric, error) {
	const query = `
		SELECT metric_date, metric_name, dimension, metric_value, computed_at
		FROM business_metrics
		WHERE metric_date = $1
		ORDER BY metric_name, dimension
	`

	rows, err := r.db.QueryContext(ctx, query, metricDate)
	if err != nil {
		return nil, fmt.Errorf("query business metrics by date: %w", err)
	}
	defer rows.Close()

	result := make([]BusinessMetric, 0, 16)
	for rows.Next() {
		var item BusinessMetric
		if err := rows.Scan(&item.MetricDate, &item.MetricName, &item.Dimension, &item.Value, &item.ComputedAt); err != nil {
			return nil, fmt.Errorf("scan business metric row: %w", err)
		}
		result = append(result, item)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate business metric rows: %w", err)
	}

	return result, nil
}

