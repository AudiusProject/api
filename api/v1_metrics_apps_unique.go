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

func (app *ApiServer) v1MetricsAppsUnique(c *fiber.Ctx) error {
	queryParams := GetMetricsAppsQueryParams{}
	if err := app.ParseAndValidateQueryParams(c, &queryParams); err != nil {
		return err
	}
	routeParams := GetAggregateAppMetricsRouteParams{}
	if err := c.ParamsParser(&routeParams); err != nil {
		return err
	}

	var dateRangeClause string
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

	sql := fmt.Sprintf(`
		SELECT 
			COALESCE(developer_apps.name, api_metrics_apps_unique.app_name) AS name,
			api_metrics_apps_unique.app_name AS identifier,
			hll_sketch,
			unique_count,
			total_count
		FROM api_metrics_apps_unique
		LEFT JOIN developer_apps ON developer_apps.address = api_metrics_apps_unique.app_name
		WHERE %s
		ORDER BY api_metrics_apps_unique.app_name
	`, dateRangeClause)

	rows, err := app.pool.Query(c.Context(), sql)
	if err != nil {
		return fmt.Errorf("failed to query app unique metrics: %w", err)
	}
	defer rows.Close()

	type metricRow struct {
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

	appMap := make(map[string][]hll.SketchRow)
	nameMap := make(map[string]string) // Maps identifier to display name
	for _, row := range metricRows {
		appMap[row.Identifier] = append(appMap[row.Identifier], hll.SketchRow{
			SketchData:  row.HllSketch,
			UniqueCount: row.UniqueCount,
			TotalCount:  row.TotalCount,
		})
		// Store the display name for this identifier
		nameMap[row.Identifier] = row.Name
	}

	result := make([]AppUniqueMetric, 0, len(appMap))
	for identifier, sketchRows := range appMap {
		merged, err := hll.MergeSketches(sketchRows, 12)
		if err != nil {
			return fmt.Errorf("failed to merge sketches for identifier %s: %w", identifier, err)
		}

		// Use the display name from the join, or fall back to identifier
		displayName := nameMap[identifier]
		if displayName == "" {
			displayName = identifier
		}

		result = append(result, AppUniqueMetric{
			Name:        displayName,
			UniqueCount: int64(merged.UniqueCount),
		})
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].UniqueCount > result[j].UniqueCount
	})

	if len(result) > queryParams.Limit {
		result = result[:queryParams.Limit]
	}

	return c.JSON(fiber.Map{
		"data": result,
	})
}
