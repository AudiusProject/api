package api

import (
	"fmt"

	"api.audius.co/hll"
	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"
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

type GetMetricsRoutesRouteParams struct {
	TimeRange string `params:"time_range" default:"all_time" validate:"oneof=week month year all_time"`
}

type GetMetricsRoutesQueryParams struct {
	BucketSize string `query:"bucket_size" default:"day" validate:"oneof=day week month year"`
}

func (app *ApiServer) v1MetricsRoutes(c *fiber.Ctx) error {
	routeParams := GetMetricsRoutesRouteParams{}
	if err := c.ParamsParser(&routeParams); err != nil {
		return err
	}

	queryParams := GetMetricsRoutesQueryParams{}
	if err := c.QueryParser(&queryParams); err != nil {
		return err
	}

	var dateRangeClause string
	var bucketClause string
	var orderBy string

	switch routeParams.TimeRange {
	case "week":
		dateRangeClause = "date >= CURRENT_DATE - INTERVAL '7 days' AND date < CURRENT_DATE"
	case "month":
		dateRangeClause = "date >= CURRENT_DATE - INTERVAL '30 days' AND date < CURRENT_DATE"
	case "year":
		dateRangeClause = "date >= CURRENT_DATE - INTERVAL '365 days' AND date < CURRENT_DATE"
	default: // all_time
		dateRangeClause = "date < CURRENT_DATE"
	}

	switch queryParams.BucketSize {
	case "week":
		bucketClause = "date_trunc('week', date)::date::text AS bucket"
		orderBy = "bucket ASC"
		// Exclude current incomplete week
		dateRangeClause = dateRangeClause + " AND date_trunc('week', date) < date_trunc('week', CURRENT_DATE)"
	case "month":
		bucketClause = "date_trunc('month', date)::date::text AS bucket"
		orderBy = "bucket ASC"
		// Exclude current incomplete month
		dateRangeClause = dateRangeClause + " AND date_trunc('month', date) < date_trunc('month', CURRENT_DATE)"
	default: // day
		bucketClause = "date::text AS bucket"
		orderBy = "bucket ASC"
	}

	sql := fmt.Sprintf(`
		SELECT 
			%s,
			hll_sketch,
			unique_count,
			total_count
		FROM api_metrics_counts
		WHERE %s
		ORDER BY %s
	`, bucketClause, dateRangeClause, orderBy)

	rows, err := app.pool.Query(c.Context(), sql)
	if err != nil {
		return fmt.Errorf("failed to query metrics: %w", err)
	}

	type metricRow struct {
		Bucket      string `db:"bucket"`
		HllSketch   []byte `db:"hll_sketch"`
		UniqueCount int64  `db:"unique_count"`
		TotalCount  int64  `db:"total_count"`
	}

	metricRows, err := pgx.CollectRows(rows, pgx.RowToStructByName[metricRow])
	if err != nil {
		return fmt.Errorf("failed to collect metrics: %w", err)
	}

	// Group rows by bucket
	bucketMap := make(map[string][]hll.SketchRow)
	bucketOrder := []string{}

	for _, row := range metricRows {
		if _, exists := bucketMap[row.Bucket]; !exists {
			bucketOrder = append(bucketOrder, row.Bucket)
		}

		// Include all rows, even those with null sketches
		bucketMap[row.Bucket] = append(bucketMap[row.Bucket], hll.SketchRow{
			SketchData:  row.HllSketch, // may be nil
			UniqueCount: row.UniqueCount,
			TotalCount:  row.TotalCount,
		})
	}

	// Merge sketches for each bucket
	result := make([]TimestampedRouteMetric, 0, len(bucketOrder))
	for _, bucket := range bucketOrder {
		merged, err := hll.MergeSketches(bucketMap[bucket], 12)
		if err != nil {
			return fmt.Errorf("failed to merge sketches for bucket %s: %w", bucket, err)
		}

		result = append(result, TimestampedRouteMetric{
			Timestamp: bucket,
			RouteMetric: RouteMetric{
				UniqueCount:       int(merged.UniqueCount),
				SummedUniqueCount: int(merged.SummedUniqueCount),
				TotalCount:        int(merged.TotalCount),
			},
		})
	}

	return c.JSON(fiber.Map{
		"data": result,
	})
}

type GetMetricsRoutesTrailingRouteParams struct {
	TimeRange string `params:"time_range" default:"month"`
}

func (app *ApiServer) v1MetricsRoutesTrailing(c *fiber.Ctx) error {
	params := GetMetricsRoutesTrailingRouteParams{}
	if err := c.ParamsParser(&params); err != nil {
		return err
	}

	var dateRangeClause string
	switch params.TimeRange {
	case "week":
		dateRangeClause = "date >= CURRENT_DATE - INTERVAL '7 days' AND date < CURRENT_DATE"
	case "year":
		dateRangeClause = "date >= CURRENT_DATE - INTERVAL '365 days' AND date < CURRENT_DATE"
	default: // month
		dateRangeClause = "date >= CURRENT_DATE - INTERVAL '30 days' AND date < CURRENT_DATE"
	}

	// Query to get the date range and fetch HLL sketches
	sql := fmt.Sprintf(`
		SELECT 
			hll_sketch,
			unique_count,
			total_count
		FROM api_metrics_counts
		WHERE %s
		ORDER BY date ASC
	`, dateRangeClause)

	rows, err := app.pool.Query(c.Context(), sql)
	if err != nil {
		return fmt.Errorf("failed to query metrics: %w", err)
	}
	defer rows.Close()

	// Use the helper to merge all HLL sketches
	merged, err := hll.MergeSketchesFromRows(rows, 12)
	if err != nil {
		return fmt.Errorf("failed to merge sketches: %w", err)
	}

	metric := RouteMetric{
		UniqueCount:       int(merged.UniqueCount),
		SummedUniqueCount: int(merged.SummedUniqueCount),
		TotalCount:        int(merged.TotalCount),
	}

	return c.JSON(fiber.Map{
		"data": metric,
	})
}
