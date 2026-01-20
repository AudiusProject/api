package api

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMetricsAppsUnique(t *testing.T) {
	app := testAppWithFixtures(t)

	tests := []struct {
		name      string
		timeRange string
		limit     int
	}{
		{"all_time", "all_time", 100},
		{"month", "month", 50},
		{"week", "week", 25},
		{"year", "year", 100},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			url := "/v1/metrics/aggregates/apps/" + tt.timeRange + "/unique"
			if tt.limit != 100 {
				url += fmt.Sprintf("?limit=%d", tt.limit)
			}

			var response struct {
				Data []struct {
					Name        string `json:"name"`
					UniqueCount int64  `json:"unique_count"`
				} `json:"data"`
			}

			status, _ := testGet(t, app, url, &response)
			assert.Equal(t, 200, status)
			assert.NotNil(t, response.Data)

			for _, appMetric := range response.Data {
				assert.NotEmpty(t, appMetric.Name, "App name should not be empty")
				assert.GreaterOrEqual(t, appMetric.UniqueCount, int64(0), "Unique count should be non-negative")
			}

			assert.LessOrEqual(t, len(response.Data), tt.limit, "Response should not exceed limit")
		})
	}
}
