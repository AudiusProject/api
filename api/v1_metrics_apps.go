package api

import (
	"bridgerton.audius.co/api/dbv1"
	"github.com/gofiber/fiber/v2"
)

type GetAggregateAppMetricsRouteParams struct {
	TimeRange string `params:"time_range" default:"all_time" validate:"oneof=week month year all_time"`
}

type GetMetricsAppsQueryParams struct {
	Limit int `query:"limit" default:"100" validate:"min=1,max=1000"`
}

type AppMetric struct {
	Name  string `json:"name"`
	Count int64  `json:"count"`
}

func (app *ApiServer) v1MetricsApps(c *fiber.Ctx) error {
	queryParams := GetMetricsAppsQueryParams{}
	if err := app.ParseAndValidateQueryParams(c, &queryParams); err != nil {
		return err
	}
	routeParams := GetAggregateAppMetricsRouteParams{}
	if err := c.ParamsParser(&routeParams); err != nil {
		return err
	}

	metrics, err := app.queries.GetAggregateAppMetrics(c.Context(), dbv1.GetAggregateAppMetricsParams{
		TimeRange: routeParams.TimeRange,
		LimitVal:  int32(queryParams.Limit),
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
