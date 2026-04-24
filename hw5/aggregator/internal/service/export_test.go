package service

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/online-cinema/aggregator/internal/repository"
)

func TestBuildExportObjectKey(t *testing.T) {
	d := time.Date(2026, 4, 24, 12, 30, 0, 0, time.UTC)
	key := buildExportObjectKey("daily", d)
	require.Equal(t, "daily/2026-04-24/aggregates.json", key)
}

func TestBuildExportPayload(t *testing.T) {
	d := time.Date(2026, 4, 24, 0, 0, 0, 0, time.UTC)
	rows := []repository.BusinessMetric{
		{
			MetricDate: d,
			MetricName: "dau",
			Dimension:  "",
			Value:      42,
			ComputedAt: time.Date(2026, 4, 24, 1, 2, 3, 0, time.UTC),
		},
	}

	payload, err := buildExportPayload(d, rows)
	require.NoError(t, err)

	var decoded []map[string]any
	require.NoError(t, json.Unmarshal(payload, &decoded))
	require.Len(t, decoded, 1)
	require.Equal(t, "2026-04-24", decoded[0]["metric_date"])
	require.Equal(t, "dau", decoded[0]["metric_name"])
	require.Equal(t, "2026-04-24T01:02:03Z", decoded[0]["computed_at"])
}

