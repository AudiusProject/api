package api

import (
	"bridgerton.audius.co/api/dbv1"
	"github.com/gofiber/fiber/v2"
)

type RouteMetric struct {
	Timestamp         string
	UniqueCount       int64
	SummedUniqueCount int64
	TotalCount        int64
}

func (app *ApiServer) v1MetricsRoutes(c *fiber.Ctx) error {
	timeRange, err := app.paramTimeRange(c, "time_range", "all_time")
	if err != nil {
		return err
	}

	bucketSize, err := app.queryDateBucket(c, "bucket_size", "day")
	if err != nil {
		return err
	}

	metrics, err := app.queries.GetAggregateRouteMetrics(c.Context(), dbv1.GetAggregateRouteMetricsParams{
		TimeRange:  timeRange,
		BucketSize: bucketSize,
	})
	if err != nil {
		return err
	}

	result := make([]map[string]interface{}, len(metrics))
	for i, metric := range metrics {
		// SQL query now returns the date already in the correct format
		result[i] = map[string]interface{}{
			"timestamp":           metric.Timestamp,
			"unique_count":        metric.UniqueCount,
			"summed_unique_count": metric.SummedUniqueCount,
			"total_count":         metric.TotalCount,
		}
	}

	return c.JSON(fiber.Map{
		"data": result,
	})
}

func (app *ApiServer) v1MetricsRoutesTrailing(c *fiber.Ctx) error {
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
