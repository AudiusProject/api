package api

import (
	"fmt"
	"sort"

	"api.audius.co/hll"
	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"
)

type AppUniqueMetric struct {
	Name        string `json:"name"`
	UniqueCount int64  `json:"unique_count"`
}

type TimestampedAppUniqueMetric struct {
	AppUniqueMetric
	Timestamp string `json:"timestamp"`
}

type GetMetricsAppsUniqueQueryParams struct {
	Limit      int    `query:"limit" default:"100" validate:"min=1,max=1000"`
	BucketSize string `query:"bucket_size" default:"day" validate:"oneof=day week month year"`
	AppName    string `query:"app_name"` // Optional: filter by app name or identifier
}

func (app *ApiServer) v1MetricsAppsUnique(c *fiber.Ctx) error {
	queryParams := GetMetricsAppsUniqueQueryParams{}
	if err := app.ParseAndValidateQueryParams(c, &queryParams); err != nil {
		return err
	}
	routeParams := GetAggregateAppMetricsRouteParams{}
	if err := c.ParamsParser(&routeParams); err != nil {
		return err
	}

	var dateRangeClause string
	var bucketClause string
	var orderBy string
	var appNameFilter string

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
		orderBy = "bucket ASC, api_metrics_apps_unique.app_name"
		// Exclude current incomplete week
		dateRangeClause = dateRangeClause + " AND date_trunc('week', date) < date_trunc('week', CURRENT_DATE)"
	case "month":
		bucketClause = "date_trunc('month', date)::date::text AS bucket"
		orderBy = "bucket ASC, api_metrics_apps_unique.app_name"
		// Exclude current incomplete month
		dateRangeClause = dateRangeClause + " AND date_trunc('month', date) < date_trunc('month', CURRENT_DATE)"
	default: // day
		bucketClause = "date::text AS bucket"
		orderBy = "bucket ASC, api_metrics_apps_unique.app_name"
	}

	// Add app_name filter if provided
	// Filter by identifier (app_name column) which could be api_key or app_name
	if queryParams.AppName != "" {
		appNameFilter = " AND api_metrics_apps_unique.app_name = $1"
	}

	whereClause := dateRangeClause + appNameFilter

	sql := fmt.Sprintf(`
		SELECT 
			%s,
			COALESCE(developer_apps.name, api_metrics_apps_unique.app_name) AS name,
			api_metrics_apps_unique.app_name AS identifier,
			hll_sketch,
			unique_count,
			total_count
		FROM api_metrics_apps_unique
		LEFT JOIN developer_apps ON developer_apps.address = api_metrics_apps_unique.app_name
		WHERE %s
		ORDER BY %s
	`, bucketClause, whereClause, orderBy)

	var rows pgx.Rows
	var err error
	if queryParams.AppName != "" {
		rows, err = app.pool.Query(c.Context(), sql, queryParams.AppName)
	} else {
		rows, err = app.pool.Query(c.Context(), sql)
	}
	if err != nil {
		return fmt.Errorf("failed to query app unique metrics: %w", err)
	}
	defer rows.Close()

	type metricRow struct {
		Bucket      string `db:"bucket"`
		Name        string `db:"name"`
		Identifier  string `db:"identifier"`
		HllSketch   []byte `db:"hll_sketch"`
		UniqueCount int64  `db:"unique_count"`
		TotalCount  int64  `db:"total_count"`
	}

	metricRows, err := pgx.CollectRows(rows, pgx.RowToStructByName[metricRow])
	if err != nil {
		return fmt.Errorf("failed to collect app unique metrics: %w", err)
	}

	// Group rows by (bucket, identifier) tuple
	type bucketAppKey struct {
		Bucket     string
		Identifier string
	}
	bucketAppMap := make(map[bucketAppKey][]hll.SketchRow)
	nameMap := make(map[string]string) // Maps identifier to display name
	bucketOrder := []string{}
	seenBuckets := make(map[string]bool)

	for _, row := range metricRows {
		key := bucketAppKey{Bucket: row.Bucket, Identifier: row.Identifier}
		bucketAppMap[key] = append(bucketAppMap[key], hll.SketchRow{
			SketchData:  row.HllSketch,
			UniqueCount: row.UniqueCount,
			TotalCount:  row.TotalCount,
		})
		// Store the display name for this identifier
		nameMap[row.Identifier] = row.Name
		if !seenBuckets[row.Bucket] {
			bucketOrder = append(bucketOrder, row.Bucket)
			seenBuckets[row.Bucket] = true
		}
	}

	// Merge sketches for each (bucket, app) combination
	result := make([]TimestampedAppUniqueMetric, 0)

	// When no app_name is provided, we need to ensure all buckets are included
	// by limiting apps per bucket rather than applying a global limit
	if queryParams.AppName == "" {
		type bucketAppResult struct {
			Timestamp string
			AppUniqueMetric
		}
		bucketResults := make(map[string][]bucketAppResult)

		for _, bucket := range bucketOrder {
			bucketApps := make([]bucketAppResult, 0)
			for key, sketchRows := range bucketAppMap {
				if key.Bucket != bucket {
					continue
				}
				merged, err := hll.MergeSketches(sketchRows, 12)
				if err != nil {
					return fmt.Errorf("failed to merge sketches for bucket %s, identifier %s: %w", bucket, key.Identifier, err)
				}

				// Use the display name from the join, or fall back to identifier
				displayName := nameMap[key.Identifier]
				if displayName == "" {
					displayName = key.Identifier
				}

				bucketApps = append(bucketApps, bucketAppResult{
					Timestamp: bucket,
					AppUniqueMetric: AppUniqueMetric{
						Name:        displayName,
						UniqueCount: int64(merged.UniqueCount),
					},
				})
			}

			// Sort apps within bucket by unique count descending
			sort.Slice(bucketApps, func(i, j int) bool {
				return bucketApps[i].UniqueCount > bucketApps[j].UniqueCount
			})

			// Calculate apps per bucket to ensure all buckets are included
			// Distribute limit across all buckets, with minimum 1 app per bucket
			// This ensures we get data for all dates, not just the first few dates
			appsPerBucket := queryParams.Limit / len(bucketOrder)
			if appsPerBucket < 1 {
				// If we have more buckets than the limit, we'll still include 1 app per bucket
				// and let the final limit trim if needed (but at least all buckets are represented)
				appsPerBucket = 1
			}
			if len(bucketApps) > appsPerBucket {
				bucketApps = bucketApps[:appsPerBucket]
			}

			bucketResults[bucket] = bucketApps
		}

		// Flatten results maintaining bucket order
		for _, bucket := range bucketOrder {
			if apps, ok := bucketResults[bucket]; ok {
				for _, app := range apps {
					result = append(result, TimestampedAppUniqueMetric{
						Timestamp:       app.Timestamp,
						AppUniqueMetric: app.AppUniqueMetric,
					})
				}
			}
		}
	} else {
		// When app_name is provided, process all buckets for that app
		for _, bucket := range bucketOrder {
			for key, sketchRows := range bucketAppMap {
				if key.Bucket != bucket {
					continue
				}
				merged, err := hll.MergeSketches(sketchRows, 12)
				if err != nil {
					return fmt.Errorf("failed to merge sketches for bucket %s, identifier %s: %w", bucket, key.Identifier, err)
				}

				// Use the display name from the join, or fall back to identifier
				displayName := nameMap[key.Identifier]
				if displayName == "" {
					displayName = key.Identifier
				}

				result = append(result, TimestampedAppUniqueMetric{
					Timestamp: bucket,
					AppUniqueMetric: AppUniqueMetric{
						Name:        displayName,
						UniqueCount: int64(merged.UniqueCount),
					},
				})
			}
		}
	}

	// Sort by timestamp then by unique count descending
	sort.Slice(result, func(i, j int) bool {
		if result[i].Timestamp != result[j].Timestamp {
			return result[i].Timestamp < result[j].Timestamp
		}
		return result[i].UniqueCount > result[j].UniqueCount
	})

	// Apply limit
	if len(result) > queryParams.Limit {
		result = result[:queryParams.Limit]
	}

	return c.JSON(fiber.Map{
		"data": result,
	})
}
