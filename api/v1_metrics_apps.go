package api

import (
	"bridgerton.audius.co/api/dbv1"
	"github.com/gofiber/fiber/v2"
)

type GetMetricsAppsParams struct {
	TimeRange string `query:"time_range" default:"all_time" validate:"oneof=week month year all_time"`
	Limit     int    `query:"limit" default:"100" validate:"min=1,max=100"`
}

type AppMetric struct {
	Name  string `json:"name"`
	Count int64  `json:"count"`
}

func (app *ApiServer) v1MetricsApps(c *fiber.Ctx) error {
	params := GetMetricsAppsParams{}
	if err := app.ParseAndValidateQueryParams(c, &params); err != nil {
		return err
	}

	metrics, err := app.queries.GetAggregateAppMetrics(c.Context(), dbv1.GetAggregateAppMetricsParams{
		TimeRange: params.TimeRange,
		LimitVal:  int32(params.Limit),
	})
	if err != nil {
		return err
	}

	result := make([]AppMetric, len(metrics))
	for i, metric := range metrics {
		result[i] = AppMetric{
			Name:  metric.Name,
			Count: metric.Count,
		}
	}

	return c.JSON(fiber.Map{
		"data": result,
	})
}
