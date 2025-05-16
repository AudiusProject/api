package api

import (
	"strconv"
	"time"

	"bridgerton.audius.co/api/dbv1"
	"github.com/gofiber/fiber/v2"
)

const oneWeekInHours = 168

func (app *ApiServer) v1MetricsPlays(c *fiber.Ctx) error {
	limit := c.QueryInt("limit", oneWeekInHours)
	if limit > oneWeekInHours {
		limit = oneWeekInHours
	}

	startTime := time.Unix(int64(c.QueryInt("start_time", 0)), 0)
	bucketSize, err := app.queryDateBucket(c, "bucket_size", "hour")
	if err != nil {
		return err
	}

	metrics, err := app.queries.GetPlays(c.Context(), dbv1.GetPlaysParams{
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
