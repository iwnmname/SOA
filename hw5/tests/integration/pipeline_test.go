package integration

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"testing"
	"time"

	_ "github.com/ClickHouse/clickhouse-go/v2"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type eventRequest struct {
	EventID         string `json:"event_id"`
	UserID          string `json:"user_id"`
	MovieID         string `json:"movie_id"`
	EventType       string `json:"event_type"`
	DeviceType      string `json:"device_type"`
	SessionID       string `json:"session_id"`
	ProgressSeconds int32  `json:"progress_seconds"`
	Timestamp       string `json:"timestamp"`
}

type eventResponse struct {
	EventID string `json:"event_id"`
	Status  string `json:"status"`
}

type clickhouseEvent struct {
	EventID         string
	UserID          string
	MovieID         string
	EventType       string
	TimestampMs     int64
	DeviceType      string
	SessionID       string
	ProgressSeconds int32
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getProducerURL() string {
	return getEnv("PRODUCER_URL", "http://localhost:8080")
}

func getClickHouseDSN() string {
	return getEnv("CLICKHOUSE_DSN", "clickhouse://app:app@localhost:9000/default")
}

func publishEvent(t *testing.T, event eventRequest) eventResponse {
	t.Helper()

	body, err := json.Marshal(event)
	require.NoError(t, err)

	resp, err := http.Post(
		getProducerURL()+"/api/v1/events",
		"application/json",
		bytes.NewReader(body),
	)
	require.NoError(t, err)
	defer resp.Body.Close()

	require.Equal(t, http.StatusOK, resp.StatusCode)

	var result eventResponse
	err = json.NewDecoder(resp.Body).Decode(&result)
	require.NoError(t, err)

	return result
}

func waitForEventInClickHouse(t *testing.T, db *sql.DB, eventID string, timeout time.Duration) *clickhouseEvent {
	t.Helper()

	deadline := time.Now().Add(timeout)
	query := `
		SELECT
			event_id,
			user_id,
			movie_id,
			event_type,
			timestamp_ms,
			device_type,
			session_id,
			progress_seconds
		FROM movie_events
		WHERE event_id = ?
		LIMIT 1
	`

	for time.Now().Before(deadline) {
		row := db.QueryRow(query, eventID)

		var ev clickhouseEvent
		err := row.Scan(
			&ev.EventID,
			&ev.UserID,
			&ev.MovieID,
			&ev.EventType,
			&ev.TimestampMs,
			&ev.DeviceType,
			&ev.SessionID,
			&ev.ProgressSeconds,
		)
		if err == nil {
			return &ev
		}

		time.Sleep(500 * time.Millisecond)
	}

	t.Fatalf("event %s not found in ClickHouse after %v", eventID, timeout)
	return nil
}

func connectClickHouse(t *testing.T) *sql.DB {
	t.Helper()

	dsn := getClickHouseDSN()
	db, err := sql.Open("clickhouse", dsn)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	for ctx.Err() == nil {
		if err := db.PingContext(ctx); err == nil {
			return db
		}
		time.Sleep(1 * time.Second)
	}

	require.NoError(t, db.Ping())
	return db
}

func TestFullPipeline_SingleEvent(t *testing.T) {
	db := connectClickHouse(t)
	defer db.Close()

	eventID := uuid.New().String()
	sessionID := uuid.New().String()

	input := eventRequest{
		EventID:         eventID,
		UserID:          "test-user-001",
		MovieID:         "test-movie-001",
		EventType:       "VIEW_STARTED",
		DeviceType:      "DESKTOP",
		SessionID:       sessionID,
		ProgressSeconds: 0,
		Timestamp:       time.Now().UTC().Format(time.RFC3339),
	}

	resp := publishEvent(t, input)
	assert.Equal(t, eventID, resp.EventID)
	assert.Equal(t, "published", resp.Status)

	result := waitForEventInClickHouse(t, db, eventID, 60*time.Second)

	assert.Equal(t, eventID, result.EventID)
	assert.Equal(t, "test-user-001", result.UserID)
	assert.Equal(t, "test-movie-001", result.MovieID)
	assert.Equal(t, "VIEW_STARTED", result.EventType)
	assert.Equal(t, "DESKTOP", result.DeviceType)
	assert.Equal(t, sessionID, result.SessionID)
	assert.Equal(t, int32(0), result.ProgressSeconds)
	assert.Greater(t, result.TimestampMs, int64(0))
}

func TestFullPipeline_SessionSequence(t *testing.T) {
	db := connectClickHouse(t)
	defer db.Close()

	sessionID := uuid.New().String()
	userID := "test-user-seq-" + uuid.New().String()[:8]
	movieID := "test-movie-seq-001"

	events := []eventRequest{
		{
			EventID:         uuid.New().String(),
			UserID:          userID,
			MovieID:         movieID,
			EventType:       "VIEW_STARTED",
			DeviceType:      "MOBILE",
			SessionID:       sessionID,
			ProgressSeconds: 0,
		},
		{
			EventID:         uuid.New().String(),
			UserID:          userID,
			MovieID:         movieID,
			EventType:       "VIEW_PAUSED",
			DeviceType:      "MOBILE",
			SessionID:       sessionID,
			ProgressSeconds: 120,
		},
		{
			EventID:         uuid.New().String(),
			UserID:          userID,
			MovieID:         movieID,
			EventType:       "VIEW_RESUMED",
			DeviceType:      "MOBILE",
			SessionID:       sessionID,
			ProgressSeconds: 120,
		},
		{
			EventID:         uuid.New().String(),
			UserID:          userID,
			MovieID:         movieID,
			EventType:       "VIEW_FINISHED",
			DeviceType:      "MOBILE",
			SessionID:       sessionID,
			ProgressSeconds: 5400,
		},
	}

	for _, ev := range events {
		resp := publishEvent(t, ev)
		assert.Equal(t, "published", resp.Status)
		time.Sleep(100 * time.Millisecond)
	}

	lastEventID := events[len(events)-1].EventID
	waitForEventInClickHouse(t, db, lastEventID, 60*time.Second)

	query := `
		SELECT count()
		FROM movie_events
		WHERE session_id = ?
	`
	var count int
	err := db.QueryRow(query, sessionID).Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 4, count)

	queryOrder := `
		SELECT event_type
		FROM movie_events
		WHERE session_id = ?
		ORDER BY timestamp_ms ASC
	`
	rows, err := db.Query(queryOrder, sessionID)
	require.NoError(t, err)
	defer rows.Close()

	expectedOrder := []string{"VIEW_STARTED", "VIEW_PAUSED", "VIEW_RESUMED", "VIEW_FINISHED"}
	var actualOrder []string
	for rows.Next() {
		var et string
		require.NoError(t, rows.Scan(&et))
		actualOrder = append(actualOrder, et)
	}
	assert.Equal(t, expectedOrder, actualOrder)
}

func TestFullPipeline_AllEventTypes(t *testing.T) {
	db := connectClickHouse(t)
	defer db.Close()

	eventTypes := []struct {
		Type     string
		Progress int32
	}{
		{"VIEW_STARTED", 0},
		{"VIEW_FINISHED", 3600},
		{"VIEW_PAUSED", 1800},
		{"VIEW_RESUMED", 1800},
		{"LIKED", 0},
		{"SEARCHED", 0},
	}

	for _, et := range eventTypes {
		t.Run(et.Type, func(t *testing.T) {
			eventID := uuid.New().String()
			input := eventRequest{
				EventID:         eventID,
				UserID:          "test-user-types-" + uuid.New().String()[:8],
				MovieID:         "test-movie-types-001",
				EventType:       et.Type,
				DeviceType:      "TV",
				SessionID:       uuid.New().String(),
				ProgressSeconds: et.Progress,
			}

			resp := publishEvent(t, input)
			assert.Equal(t, "published", resp.Status)

			result := waitForEventInClickHouse(t, db, eventID, 60*time.Second)
			assert.Equal(t, et.Type, result.EventType)
			assert.Equal(t, et.Progress, result.ProgressSeconds)
		})
	}
}

func TestFullPipeline_AllDeviceTypes(t *testing.T) {
	db := connectClickHouse(t)
	defer db.Close()

	deviceTypes := []string{"MOBILE", "DESKTOP", "TV", "TABLET"}

	for _, dt := range deviceTypes {
		t.Run(dt, func(t *testing.T) {
			eventID := uuid.New().String()
			input := eventRequest{
				EventID:         eventID,
				UserID:          "test-user-devices-" + uuid.New().String()[:8],
				MovieID:         "test-movie-devices-001",
				EventType:       "VIEW_STARTED",
				DeviceType:      dt,
				SessionID:       uuid.New().String(),
				ProgressSeconds: 0,
			}

			resp := publishEvent(t, input)
			assert.Equal(t, "published", resp.Status)

			result := waitForEventInClickHouse(t, db, eventID, 60*time.Second)
			assert.Equal(t, dt, result.DeviceType)
		})
	}
}

func TestFullPipeline_ValidationError(t *testing.T) {
	body := []byte(`{"event_id": "not-a-uuid", "user_id": "", "event_type": "INVALID"}`)

	resp, err := http.Post(
		getProducerURL()+"/api/v1/events",
		"application/json",
		bytes.NewReader(body),
	)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestFullPipeline_Idempotency(t *testing.T) {
	db := connectClickHouse(t)
	defer db.Close()

	eventID := uuid.New().String()
	input := eventRequest{
		EventID:         eventID,
		UserID:          "test-user-idempotent",
		MovieID:         "test-movie-idempotent",
		EventType:       "VIEW_STARTED",
		DeviceType:      "DESKTOP",
		SessionID:       uuid.New().String(),
		ProgressSeconds: 0,
	}

	for i := 0; i < 3; i++ {
		resp := publishEvent(t, input)
		assert.Equal(t, "published", resp.Status)
	}

	waitForEventInClickHouse(t, db, eventID, 60*time.Second)

	time.Sleep(5 * time.Second)

	query := fmt.Sprintf(
		"SELECT count() FROM movie_events FINAL WHERE event_id = '%s'",
		eventID,
	)
	var count int
	err := db.QueryRow(query).Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 1, count)
}
