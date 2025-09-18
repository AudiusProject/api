package api

import (
	"encoding/json"

	"github.com/gofiber/fiber/v2"
)

type CollectiblesRow struct {
	Data json.RawMessage `json:"data"`
}

func (app *ApiServer) v1UsersCollectibles(c *fiber.Ctx) error {
	userId := app.getUserId(c)

	sql := `
		SELECT data FROM collectibles
		WHERE user_id = $1
		LIMIT 1
	`

	var collectible CollectiblesRow

	err := app.pool.QueryRow(c.Context(), sql, userId).Scan(&collectible.Data)
	if err != nil {
		return err
	}

	return c.JSON(fiber.Map{
		"data": collectible.Data,
	})
}
