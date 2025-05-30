package api

import (
	"bridgerton.audius.co/api/dbv1"
	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"
)

func (app *ApiServer) getChatPermissions(c *fiber.Ctx) error {
	sql := `
	SELECT
		user_id,
		string_agg(permits, ',') as permits,
		chat_allowed(@current_user_id, user_id) AS current_user_has_permission
	FROM chat_permissions
	WHERE user_id = ANY(@user_ids)
	AND allowed = TRUE
	GROUP BY user_id
	;`

	wallet := app.getAuthedWallet(c)
	userId, err := app.getUserIDFromWallet(c.Context(), wallet)
	if err != nil {
		return err
	}

	userIds := decodeIdList(c)
	if len(userIds) == 0 {
		return fiber.NewError(fiber.StatusBadRequest, "invalid id parameter")
	}

	rawRows, err := app.pool.Query(c.Context(), sql, pgx.NamedArgs{
		"current_user_id": userId,
		"user_ids":        userIds,
	})
	if err != nil && err != pgx.ErrNoRows {
		return err
	}

	rows, err := pgx.CollectRows(rawRows, pgx.RowToStructByName[dbv1.ChatPermissionsRow])
	if err != nil {
		return err
	}

	return c.JSON(CommsResponse{
		Data: rows,
		Health: CommsHealth{
			IsHealthy: true,
		},
	})
}
