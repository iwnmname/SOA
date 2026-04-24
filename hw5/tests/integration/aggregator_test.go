package integration

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type aggregateRunResponse struct {
	ProcessedRawEvents int64  `json:"processed_raw_events"`
	SavedMetrics       int    `json:"saved_metrics"`
	Duration           int64  `json:"duration"`
	MetricDate         string `json:"metric_date"`
}

func getAggregatorURL() string {
	if v := os.Getenv("AGGREGATOR_URL"); v != "" {
		return v
	}
	return "http://localhost:8090"
}

func getPostgresDSN() string {
	if v := os.Getenv("POSTGRES_DSN"); v != "" {
		return v
	}
	return "postgres://app:app@localhost:5432/analytics?sslmode=disable"
}

func connectPostgres(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("pgx", getPostgresDSN())
	require.NoError(t, err)

	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		if err := db.Ping(); err == nil {
			return db
		}
		time.Sleep(time.Second)
	}

	require.NoError(t, db.Ping())
	return db
}

func waitForAggregatorHealth(t *testing.T) {
	t.Helper()
	deadline := time.Now().Add(45 * time.Second)
	for time.Now().Before(deadline) {
		resp, err := http.Get(getAggregatorURL() + "/health")
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return
			}
		}
		time.Sleep(time.Second)
	}
	t.Fatalf("aggregator is not healthy at %s", getAggregatorURL())
}

func runAggregation(t *testing.T, targetDate string) aggregateRunResponse {
	t.Helper()
	resp, err := http.Post(getAggregatorURL()+"/aggregate/run?date="+targetDate, "application/json", bytes.NewReader([]byte("{}")))
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var result aggregateRunResponse
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&result))
	return result
}

func TestAggregationPipeline_RunAndUpsert(t *testing.T) {
	waitForAggregatorHealth(t)
	pg := connectPostgres(t)
	defer pg.Close()

	eventUser := "agg-user-" + time.Now().UTC().Format("150405")
	sessionID := "agg-session-" + time.Now().UTC().Format("150405")

	_ = publishEvent(t, eventRequest{
		EventID:         uuid.New().String(),
		UserID:          eventUser,
		MovieID:         "agg-movie-001",
		EventType:       "VIEW_STARTED",
		DeviceType:      "DESKTOP",
		SessionID:       sessionID,
		ProgressSeconds: 0,
	})
	_ = publishEvent(t, eventRequest{
		EventID:         uuid.New().String(),
		UserID:          eventUser,
		MovieID:         "agg-movie-001",
		EventType:       "VIEW_FINISHED",
		DeviceType:      "DESKTOP",
		SessionID:       sessionID,
		ProgressSeconds: 720,
	})

	time.Sleep(3 * time.Second)
	targetDate := time.Now().UTC().Format("2006-01-02")
	result := runAggregation(t, targetDate)
	assert.GreaterOrEqual(t, result.SavedMetrics, 1)

	var dau float64
	err := pg.QueryRow(`
		SELECT metric_value
		FROM business_metrics
		WHERE metric_name = 'dau' AND metric_date = $1
		ORDER BY computed_at DESC
		LIMIT 1
	`, targetDate).Scan(&dau)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, dau, float64(1))

	var topMovieRows int
	err = pg.QueryRow(`
		SELECT count(*)
		FROM business_metrics
		WHERE metric_name = 'top_movie_views' AND metric_date = $1
	`, targetDate).Scan(&topMovieRows)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, topMovieRows, 1)
}
