package api

import (
	"github.com/gofiber/fiber/v2"
)

func (app *ApiServer) v1RouteAggregateMetricsTrailing(c *fiber.Ctx) error {
	timeRange, err := app.paramTimeRange(c, "time_range", "all_time")
	if err != nil {
		return err
	}

	metrics, err := app.queries.GetAggregateRouteMetricsTrailing(c.Context(), timeRange)
	if err != nil {
		return err
	}

	// Format response as a single object with counts
	result := fiber.Map{
		"unique_count":        metrics.UniqueCount,
		"summed_unique_count": metrics.SummedUniqueCount,
		"total_count":         metrics.TotalCount,
	}

	return c.JSON(fiber.Map{
		"data": result,
	})
}
