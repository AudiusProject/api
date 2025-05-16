package api

import (
	"bridgerton.audius.co/api/dbv1"
	"github.com/gofiber/fiber/v2"
)

type AppMetric struct {
	Name  string `json:"name"`
	Count int64  `json:"count"`
}

func (app *ApiServer) v1AppAggregateMetrics(c *fiber.Ctx) error {
	timeRange, err := app.paramTimeRange(c, "time_range", "all_time")
	if err != nil {
		return err
	}
	limit := c.QueryInt("limit", 100)
	if limit > 100 {
		limit = 100
	}

	metrics, err := app.queries.GetAggregateAppMetrics(c.Context(), dbv1.GetAggregateAppMetricsParams{
		TimeRange: timeRange,
		LimitVal:  int32(limit),
	})
	if err != nil {
		return err
	}

	result := make([]map[string]interface{}, len(metrics))
	for i, metric := range metrics {
		result[i] = map[string]interface{}{
			"name":  metric.Name,
			"count": metric.Count,
		}
	}

	return c.JSON(fiber.Map{
		"data": result,
	})
}
