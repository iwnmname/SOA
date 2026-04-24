package integration

import (
	"bytes"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type exportRunResponse struct {
	MetricDate string `json:"metric_date"`
	Rows       int    `json:"rows"`
	Bucket     string `json:"bucket"`
	ObjectKey  string `json:"object_key"`
	ETag       string `json:"etag"`
}

func runExport(t *testing.T, targetDate string) exportRunResponse {
	t.Helper()
	resp, err := http.Post(getAggregatorURL()+"/export/run?date="+targetDate, "application/json", bytes.NewReader([]byte("{}")))
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var result exportRunResponse
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&result))
	return result
}

func TestExportPipeline_RunToS3(t *testing.T) {
	waitForAggregatorHealth(t)

	eventUser := "export-user-" + time.Now().UTC().Format("150405")
	sessionID := "export-session-" + time.Now().UTC().Format("150405")

	_ = publishEvent(t, eventRequest{
		EventID:         uuid.New().String(),
		UserID:          eventUser,
		MovieID:         "export-movie-001",
		EventType:       "VIEW_STARTED",
		DeviceType:      "MOBILE",
		SessionID:       sessionID,
		ProgressSeconds: 0,
	})
	_ = publishEvent(t, eventRequest{
		EventID:         uuid.New().String(),
		UserID:          eventUser,
		MovieID:         "export-movie-001",
		EventType:       "VIEW_FINISHED",
		DeviceType:      "MOBILE",
		SessionID:       sessionID,
		ProgressSeconds: 560,
	})

	time.Sleep(3 * time.Second)
	targetDate := time.Now().UTC().Format("2006-01-02")

	_ = runAggregation(t, targetDate)
	result := runExport(t, targetDate)

	assert.Equal(t, targetDate, result.MetricDate)
	assert.Equal(t, "movie-analytics", result.Bucket)
	assert.GreaterOrEqual(t, result.Rows, 1)
	assert.True(t, strings.HasPrefix(result.ObjectKey, "daily/"+targetDate+"/"))
	assert.NotEmpty(t, result.ETag)
}

