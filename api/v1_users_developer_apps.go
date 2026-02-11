package api

import (
	"api.audius.co/trashid"
	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"
)

type DeveloperApp struct {
	Address     string         `json:"address" db:"address"`
	UserId      trashid.HashId `json:"user_id" db:"user_id"`
	Name        string         `json:"name" db:"name"`
	Description *string        `json:"description" db:"description"`
	ImageUrl    *string        `json:"image_url" db:"image_url"`
}

type DeveloperAppWithMetrics struct {
	Address           string         `json:"address" db:"address"`
	UserId            trashid.HashId `json:"user_id" db:"user_id"`
	Name              string         `json:"name" db:"name"`
	Description       *string        `json:"description" db:"description"`
	ImageUrl          *string        `json:"image_url" db:"image_url"`
	RequestCount      int64          `json:"request_count" db:"request_count"`
	RequestCountAllTime int64         `json:"request_count_all_time" db:"request_count_all_time"`
}

func (app *ApiServer) v1UsersDeveloperApps(c *fiber.Ctx) error {
	userId := app.getUserId(c)
	includeMetrics := c.Query("include") == "metrics"

	if includeMetrics {
		return app.v1UsersDeveloperAppsWithMetrics(c, userId)
	}

	sql := `
		SELECT address, user_id, name, description, image_url
		FROM developer_apps
		WHERE user_id = @userId
		AND developer_apps.is_current = true
		AND developer_apps.is_delete = false
	`

	rows, err := app.pool.Query(c.Context(), sql, pgx.NamedArgs{
		"userId": userId,
	})
	if err != nil {
		return err
	}

	apps, err := pgx.CollectRows(rows, pgx.RowToStructByName[DeveloperApp])
	if err != nil {
		return err
	}

	return c.JSON(fiber.Map{
		"data": apps,
	})
}

func (app *ApiServer) v1UsersDeveloperAppsWithMetrics(c *fiber.Ctx, userId int32) error {
	sql := `
		SELECT 
			da.address,
			da.user_id,
			da.name,
			da.description,
			da.image_url,
			COALESCE(SUM(ama.request_count) FILTER (WHERE ama.date >= DATE_TRUNC('month', CURRENT_DATE)::date AND ama.date <= CURRENT_DATE), 0)::bigint AS request_count,
			COALESCE(SUM(ama.request_count), 0)::bigint AS request_count_all_time
		FROM developer_apps da
		LEFT JOIN api_metrics_apps ama ON ama.api_key = da.address
		WHERE da.user_id = @userId
			AND da.is_current = true
			AND da.is_delete = false
		GROUP BY da.address, da.user_id, da.name, da.description, da.image_url
	`

	rows, err := app.pool.Query(c.Context(), sql, pgx.NamedArgs{
		"userId": userId,
	})
	if err != nil {
		return err
	}

	apps, err := pgx.CollectRows(rows, pgx.RowToStructByName[DeveloperAppWithMetrics])
	if err != nil {
		return err
	}

	return c.JSON(fiber.Map{
		"data": apps,
	})
}
