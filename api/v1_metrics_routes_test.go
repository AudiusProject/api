package api

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMetricsRoutes(t *testing.T) {
	app := testAppWithFixtures(t)

	tests := []struct {
		name       string
		timeRange  string
		bucketSize string
	}{
		{"all_time_month", "all_time", "month"},
		{"all_time_week", "all_time", "week"},
		{"all_time_day", "all_time", "day"},
		{"year_month", "year", "month"},
		{"month_day", "month", "day"},
		{"week_day", "week", "day"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			url := "/v1/metrics/aggregates/routes/" + tt.timeRange + "?bucket_size=" + tt.bucketSize

			var response struct {
				Data []struct {
					UniqueCount       int    `json:"unique_count"`
					SummedUniqueCount int    `json:"summed_unique_count"`
					TotalCount        int    `json:"total_count"`
					Timestamp         string `json:"timestamp"`
				} `json:"data"`
			}

			status, _ := testGet(t, app, url, &response)
			assert.Equal(t, 200, status)
			assert.NotNil(t, response.Data)

			// Verify response structure
			for _, metric := range response.Data {
				assert.GreaterOrEqual(t, metric.UniqueCount, 0, "UniqueCount should be non-negative")
				assert.GreaterOrEqual(t, metric.SummedUniqueCount, 0, "SummedUniqueCount should be non-negative")
				assert.GreaterOrEqual(t, metric.TotalCount, 0, "TotalCount should be non-negative")
				assert.NotEmpty(t, metric.Timestamp, "Timestamp should not be empty")
			}

			// Verify data is ordered chronologically
			if len(response.Data) > 1 {
				for i := 1; i < len(response.Data); i++ {
					assert.LessOrEqual(t, response.Data[i-1].Timestamp, response.Data[i].Timestamp,
						"Data should be ordered by timestamp ascending")
				}
			}
		})
	}
}

func TestMetricsRoutesTrailing(t *testing.T) {
	app := testAppWithFixtures(t)

	tests := []struct {
		name      string
		timeRange string
	}{
		{"month", "month"},
		{"week", "week"},
		{"year", "year"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			url := "/v1/metrics/aggregates/routes/trailing/" + tt.timeRange

			var response struct {
				Data struct {
					UniqueCount       int `json:"unique_count"`
					SummedUniqueCount int `json:"summed_unique_count"`
					TotalCount        int `json:"total_count"`
				} `json:"data"`
			}

			status, _ := testGet(t, app, url, &response)
			assert.Equal(t, 200, status)

			// Verify response structure
			assert.GreaterOrEqual(t, response.Data.UniqueCount, 0, "UniqueCount should be non-negative")
			assert.GreaterOrEqual(t, response.Data.SummedUniqueCount, 0, "SummedUniqueCount should be non-negative")
			assert.GreaterOrEqual(t, response.Data.TotalCount, 0, "TotalCount should be non-negative")
		})
	}
}
