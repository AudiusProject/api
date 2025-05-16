package api

import (
	"strconv"
	"time"

	"bridgerton.audius.co/api/dbv1"
	"github.com/gofiber/fiber/v2"
)

var one_week_in_hours = 168

func (app *ApiServer) v1PlaysMetrics(c *fiber.Ctx) error {
	limit := c.QueryInt("limit", one_week_in_hours)
	if limit > one_week_in_hours {
		limit = one_week_in_hours
	}

	startTime := time.Unix(int64(c.QueryInt("start_time", 0)), 0)
	bucketSize, err := app.queryDateBucket(c, "bucket_size", "hour")
	if err != nil {
		return err
	}

	metrics, err := app.queries.GetPlaysMetrics(c.Context(), dbv1.GetPlaysMetricsParams{
		LimitVal:   int32(limit),
		StartTime:  startTime,
		BucketSize: bucketSize,
	})

	if err != nil {
		return err
	}

	result := make([]map[string]interface{}, len(metrics))
	for i, metric := range metrics {
		result[i] = map[string]interface{}{
			// API expects unix timestamp as a string, gross.
			"timestamp": strconv.FormatInt(metric.Timestamp.Unix(), 10),
			"count":     metric.Count,
		}
	}

	return c.JSON(fiber.Map{
		"data": result,
	})
}
