package api

import (
	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"
)

func (api ApiServer) getUnreadCount(c *fiber.Ctx) error {
	sql := `
	SELECT COUNT(*)
	FROM chat_member
	WHERE user_id = @user_id AND unread_count > 0
	;`

	wallet := api.getAuthedWallet(c)
	userId, err := api.getUserIDFromWallet(c.Context(), wallet)
	if err != nil {
		return err
	}

	unreadCount := 0
	err = api.pool.QueryRow(c.Context(), sql, pgx.NamedArgs{
		"user_id": userId,
	}).Scan(&unreadCount)
	if err != nil && err != pgx.ErrNoRows {
		return err
	}

	return c.JSON(CommsResponse{
		Data: unreadCount,
		Health: CommsHealth{
			IsHealthy: true,
		},
	})
}
