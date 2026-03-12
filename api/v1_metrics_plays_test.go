package api

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestV1MetricsPlays(t *testing.T) {
	app := emptyTestApp(t)

	firstTimestamp := time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC)
	secondTimestamp := time.Date(2024, 1, 1, 11, 0, 0, 0, time.UTC)

	_, err := app.pool.Exec(t.Context(), `
		INSERT INTO hourly_play_counts (hourly_timestamp, play_count)
		VALUES
			($1, 7),
			($2, 5)
	`, firstTimestamp, secondTimestamp)
	assert.NoError(t, err)

	status, body := testGet(t, app, "/v1/metrics/plays?start_time=0&bucket_size=hour&limit=10")
	assert.Equal(t, 200, status)
	jsonAssert(t, body, map[string]any{
		"data.#":           2,
		"data.0.timestamp": fmt.Sprintf("%d", secondTimestamp.Unix()),
		"data.0.count":     5,
		"data.1.timestamp": fmt.Sprintf("%d", firstTimestamp.Unix()),
		"data.1.count":     7,
	})
}

func TestV1MetricsPlaysInvalidBucketSize(t *testing.T) {
	app := emptyTestApp(t)

	status, body := testGet(t, app, "/v1/metrics/plays?bucket_size=quarter")
	assert.Equal(t, 400, status)
	jsonAssert(t, body, map[string]any{
		"error": "bucket_size is invalid",
	})
}
