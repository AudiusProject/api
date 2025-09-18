package api

import (
	"encoding/json"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"
)

type CollectiblesRow struct {
	Data json.RawMessage `json:"data"`
}

func (app *ApiServer) v1UsersCollectibles(c *fiber.Ctx) error {
	userId := app.getUserId(c)

	sql := `
		SELECT data FROM collectibles
		WHERE user_id = @userId
	`

	var collectible CollectiblesRow

	err := app.pool.QueryRow(c.Context(), sql, pgx.NamedArgs{
		"userId": userId,
	}).Scan(&collectible.Data)
	if err != nil {
		if err == pgx.ErrNoRows {
			return c.JSON(fiber.Map{
				"data": nil,
			})
		}
		return err
	}

	return c.JSON(fiber.Map{
		"data": collectible,
	})
}
