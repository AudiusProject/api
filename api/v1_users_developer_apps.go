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

func (app *ApiServer) v1UsersDeveloperApps(c *fiber.Ctx) error {
	userId := app.getUserId(c)

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
