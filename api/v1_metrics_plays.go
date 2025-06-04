package api

import (
	"fmt"
	"time"

	"bridgerton.audius.co/api/dbv1"
	"github.com/gofiber/fiber/v2"
)

type UnixTimeString time.Time

type GetMetricsPlaysParams struct {
	StartTime  int    `query:"start_time" default:"0"`
	BucketSize string `query:"bucket_size" default:"hour" validate:"oneof=minute hour day week month year"`
	Limit      int    `query:"limit" default:"168" validate:"min=1,max=168"` // 168 hours in a week
}

type PlayMetric struct {
	Timestamp UnixTimeString `json:"timestamp"`
	Count     int64          `json:"count"`
}

func (t UnixTimeString) MarshalJSON() ([]byte, error) {
	return []byte(fmt.Sprintf("\"%d\"", time.Time(t).Unix())), nil
}

func (app *ApiServer) v1MetricsPlays(c *fiber.Ctx) error {
	params := GetMetricsPlaysParams{}
	if err := app.ParseAndValidateQueryParams(c, &params); err != nil {
		return err
	}

	startTime := time.Unix(int64(params.StartTime), 0)

	metrics, err := app.queries.GetPlays(c.Context(), dbv1.GetPlaysParams{
		LimitVal:   int32(params.Limit),
		StartTime:  startTime,
		BucketSize: params.BucketSize,
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
