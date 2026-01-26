package api

import (
	"fmt"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"
)

type GetAggregateAppMetricsRouteParams struct {
	TimeRange string `params:"time_range" default:"all_time" validate:"oneof=week month year all_time"`
}

type GetMetricsAppsQueryParams struct {
	Limit int `query:"limit" default:"500" validate:"min=1,max=1000"`
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
			COALESCE(developer_apps.name, app_name) AS name,
			SUM(request_count) AS count
		FROM api_metrics_apps
		LEFT JOIN developer_apps ON developer_apps.address = api_metrics_apps.api_key
		WHERE %s
		GROUP BY COALESCE(developer_apps.name, app_name)
		ORDER BY count DESC
		LIMIT @limit
	`, dateRangeClause)

	rows, err := app.pool.Query(c.Context(), sql, pgx.NamedArgs{
		"limit": queryParams.Limit,
	})
	if err != nil {
		return fmt.Errorf("failed to query app metrics: %w", err)
	}

	result, err := pgx.CollectRows(rows, pgx.RowToStructByName[AppMetric])
	if err != nil {
		return fmt.Errorf("failed to collect app metrics: %w", err)
	}

	return c.JSON(fiber.Map{
		"data": result,
	})
}
