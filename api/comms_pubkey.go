package api

import (
	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"
)

func (api *ApiServer) getPubkey(c *fiber.Ctx) error {
	userId := api.getUserId(c)
	sql := `SELECT pubkey_base64 FROM user_pubkeys WHERE user_id = @userId`

	var pubkey string
	err := api.pool.QueryRow(c.Context(), sql, pgx.NamedArgs{
		"userId": userId,
	}).Scan(&pubkey)
	if err != nil {
		return err
	}

	return c.JSON(map[string]any{
		"data": pubkey,
	})
}
