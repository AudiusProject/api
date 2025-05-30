package api

import (
	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"
)

func (app *ApiServer) getPubkey(c *fiber.Ctx) error {
	userId := app.getUserId(c)
	sql := `SELECT pubkey_base64 FROM user_pubkeys WHERE user_id = @user_id`

	var pubkey string
	err := app.pool.QueryRow(c.Context(), sql, pgx.NamedArgs{
		"user_id": userId,
	}).Scan(&pubkey)
	if err != nil {
		return err
	}

	return c.JSON(map[string]any{
		"data": pubkey,
	})
}
