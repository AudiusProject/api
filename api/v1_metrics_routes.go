package api

import (
	"fmt"

	"api.audius.co/hll"
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

type GetMetricsRoutesQueryParams struct {
	TimeRange  string `query:"time_range" default:"month"`
	BucketSize string `query:"bucket_size" default:"day"`
}

func (app *ApiServer) v1MetricsRoutes(c *fiber.Ctx) error {
	params := GetMetricsRoutesQueryParams{}
	if err := c.QueryParser(&params); err != nil {
		return err
	}

	var dateRangeClause string
	var bucketClause string
	var orderBy string

	switch params.TimeRange {
	case "week":
		dateRangeClause = "date >= CURRENT_DATE - INTERVAL '7 days' AND date < CURRENT_DATE"
	case "year":
		dateRangeClause = "date >= CURRENT_DATE - INTERVAL '365 days' AND date < CURRENT_DATE"
	case "all_time":
		dateRangeClause = "date < CURRENT_DATE"
	default: // month
		dateRangeClause = "date >= CURRENT_DATE - INTERVAL '30 days' AND date < CURRENT_DATE"
	}

	switch params.BucketSize {
	case "week":
		bucketClause = "date_trunc('week', date)::date::text AS bucket"
		orderBy = "bucket ASC"
	case "month":
		bucketClause = "date_trunc('month', date)::date::text AS bucket"
		orderBy = "bucket ASC"
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
	defer rows.Close()

	// Group rows by bucket and merge sketches within each bucket
	result := []TimestampedRouteMetric{}
	var currentBucket string
	var bucketRows []struct {
		sketchData  []byte
		uniqueCount int64
		totalCount  int64
	}

	for rows.Next() {
		var bucket string
		var sketchData []byte
		var uniqueCount int64
		var totalCount int64

		if err := rows.Scan(&bucket, &sketchData, &uniqueCount, &totalCount); err != nil {
			return fmt.Errorf("failed to scan row: %w", err)
		}

		// If we've moved to a new bucket, process the previous bucket
		if currentBucket != "" && bucket != currentBucket {
			if len(bucketRows) > 0 {
				merged, err := mergeBucketRows(bucketRows)
				if err != nil {
					return fmt.Errorf("failed to merge bucket rows: %w", err)
				}
				result = append(result, TimestampedRouteMetric{
					Timestamp: currentBucket,
					RouteMetric: RouteMetric{
						UniqueCount:       int(merged.UniqueCount),
						SummedUniqueCount: int(merged.SummedUniqueCount),
						TotalCount:        int(merged.TotalCount),
					},
				})
			}
			bucketRows = nil
		}

		currentBucket = bucket
		bucketRows = append(bucketRows, struct {
			sketchData  []byte
			uniqueCount int64
			totalCount  int64
		}{sketchData, uniqueCount, totalCount})
	}

	// Process the last bucket
	if len(bucketRows) > 0 {
		merged, err := mergeBucketRows(bucketRows)
		if err != nil {
			return fmt.Errorf("failed to merge bucket rows: %w", err)
		}
		result = append(result, TimestampedRouteMetric{
			Timestamp: currentBucket,
			RouteMetric: RouteMetric{
				UniqueCount:       int(merged.UniqueCount),
				SummedUniqueCount: int(merged.SummedUniqueCount),
				TotalCount:        int(merged.TotalCount),
			},
		})
	}

	if err := rows.Err(); err != nil {
		return fmt.Errorf("error iterating rows: %w", err)
	}

	return c.JSON(fiber.Map{
		"data": result,
	})
}

// mergeBucketRows merges HLL sketches from multiple rows within the same bucket
func mergeBucketRows(rows []struct {
	sketchData  []byte
	uniqueCount int64
	totalCount  int64
}) (*hll.MergedMetrics, error) {
	// Convert to SketchRow format
	sketchRows := make([]hll.SketchRow, len(rows))
	for i, row := range rows {
		sketchRows[i] = hll.SketchRow{
			SketchData:  row.sketchData,
			UniqueCount: row.uniqueCount,
			TotalCount:  row.totalCount,
		}
	}

	// Merge all sketches using the helper
	return hll.MergeSketches(sketchRows, 12)
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
