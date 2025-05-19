package api

import (
	"fmt"
	"time"

	"bridgerton.audius.co/api/dbv1"
	"github.com/gofiber/fiber/v2"
)

const oneWeekInHours = 168

type UnixTimeString time.Time

type PlayMetric struct {
	Timestamp UnixTimeString `json:"timestamp"`
	Count     int64          `json:"count"`
}

func (t UnixTimeString) MarshalJSON() ([]byte, error) {
	return []byte(fmt.Sprintf("\"%d\"", time.Time(t).Unix())), nil
}

func (app *ApiServer) v1MetricsPlays(c *fiber.Ctx) error {
	limit := c.QueryInt("limit", oneWeekInHours)
	if limit > oneWeekInHours {
		return sendError(c, 400, fmt.Sprintf("Limit must be less than or equal to %d", oneWeekInHours))
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

	result := make([]PlayMetric, len(metrics))
	for i, metric := range metrics {
		result[i] = PlayMetric{
			Timestamp: UnixTimeString(metric.Timestamp),
			Count:     metric.Count,
		}
	}

	return c.JSON(fiber.Map{
		"data": result,
	})
}
