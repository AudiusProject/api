package api

import (
	"bridgerton.audius.co/api/dbv1"
	"github.com/gofiber/fiber/v2"
)

type RouteMetric struct {
	UniqueCount       int `json:"unique_count"`
	SummedUniqueCount int `json:"summed_unique_count"`
	TotalCount        int `json:"total_count"`
}

type TimestampedRouteMetric struct {
	RouteMetric
	Timestamp string `json:"timestamp"`
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

	result := make([]TimestampedRouteMetric, len(metrics))
	for i, metric := range metrics {
		result[i] = TimestampedRouteMetric{
			Timestamp: metric.Timestamp,
			RouteMetric: RouteMetric{
				UniqueCount:       int(metric.UniqueCount),
				SummedUniqueCount: int(metric.SummedUniqueCount.Int32),
				TotalCount:        int(metric.TotalCount),
			},
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
	result := RouteMetric{
		UniqueCount:       int(metrics.UniqueCount),
		SummedUniqueCount: int(metrics.SummedUniqueCount),
		TotalCount:        int(metrics.TotalCount),
	}

	return c.JSON(fiber.Map{
		"data": result,
	})
}
