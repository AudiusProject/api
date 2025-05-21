package api

import (
	"bridgerton.audius.co/trashid"
	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"
)

func (api ApiServer) getChatBlockers(c *fiber.Ctx) error {
	sql := `
	SELECT blocker_user_id AS user_id
	FROM chat_blocked_users 
	WHERE blockee_user_id = @user_id;
	`

	wallet := api.getAuthedWallet(c)
	userId, err := api.getUserIDFromWallet(c.Context(), wallet)
	if err != nil {
		return err
	}

	rawRows, err := api.pool.Query(c.Context(), sql, pgx.NamedArgs{
		"user_id": userId,
	})
	if err == pgx.ErrNoRows {
		return c.JSON(CommsResponse{
			Health: CommsHealth{
				IsHealthy: true,
			},
		})
	}
	if err != nil {
		return err
	}

	rows, err := pgx.CollectRows(rawRows, pgx.RowTo[trashid.HashId])
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
