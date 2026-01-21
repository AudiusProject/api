package api

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMetricsAppsUnique(t *testing.T) {
	app := testAppWithFixtures(t)

	tests := []struct {
		name       string
		timeRange  string
		bucketSize string
		limit      int
	}{
		{"all_time_day", "all_time", "day", 100},
		{"all_time_week", "all_time", "week", 100},
		{"all_time_month", "all_time", "month", 100},
		{"month_day", "month", "day", 50},
		{"week_day", "week", "day", 25},
		{"year_month", "year", "month", 100},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			url := "/v1/metrics/aggregates/apps/" + tt.timeRange + "/unique?bucket_size=" + tt.bucketSize
			if tt.limit != 100 {
				url += fmt.Sprintf("&limit=%d", tt.limit)
			}

			var response struct {
				Data []struct {
					Name        string `json:"name"`
					UniqueCount int64  `json:"unique_count"`
					Timestamp   string `json:"timestamp"`
				} `json:"data"`
			}

			status, _ := testGet(t, app, url, &response)
			assert.Equal(t, 200, status)
			assert.NotNil(t, response.Data)

			for _, appMetric := range response.Data {
				assert.NotEmpty(t, appMetric.Name, "App name should not be empty")
				assert.GreaterOrEqual(t, appMetric.UniqueCount, int64(0), "Unique count should be non-negative")
				assert.NotEmpty(t, appMetric.Timestamp, "Timestamp should not be empty")
			}

			// Verify data is ordered chronologically
			if len(response.Data) > 1 {
				for i := 1; i < len(response.Data); i++ {
					if response.Data[i-1].Timestamp == response.Data[i].Timestamp {
						// Same timestamp, verify ordering by unique count descending
						assert.GreaterOrEqual(t, response.Data[i-1].UniqueCount, response.Data[i].UniqueCount,
							"Data with same timestamp should be ordered by unique count descending")
					} else {
						// Different timestamps, verify chronological order
						assert.LessOrEqual(t, response.Data[i-1].Timestamp, response.Data[i].Timestamp,
							"Data should be ordered by timestamp ascending")
					}
				}
			}

			assert.LessOrEqual(t, len(response.Data), tt.limit, "Response should not exceed limit")
		})
	}
}
