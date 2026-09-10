package api

import (
	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"
)

// UnreadCountByCategory is the number of chats with unread messages, grouped
// by the current user's inbox category for each chat. Chats with no
// user_conversation_preferences row are "uncategorized".
type UnreadCountByCategory struct {
	Priority      int `json:"priority"`
	General       int `json:"general"`
	Uncategorized int `json:"uncategorized"`
}

func (app *ApiServer) getUnreadCountByCategory(c *fiber.Ctx) error {
	// Mirrors getUnreadCount's filter (unread_count > 0 on the user's
	// chat_member rows) and splits the count by category.
	sql := `
	SELECT
		COUNT(*) FILTER (WHERE ucp.category = 'priority') AS priority,
		COUNT(*) FILTER (WHERE ucp.category = 'general') AS general,
		COUNT(*) FILTER (WHERE ucp.category IS NULL) AS uncategorized
	FROM chat_member
	LEFT JOIN user_conversation_preferences ucp
		ON ucp.user_id = chat_member.user_id
		AND ucp.chat_id = chat_member.chat_id
	WHERE chat_member.user_id = @user_id AND chat_member.unread_count > 0
	;`

	wallet := app.getAuthedWallet(c)
	userId, err := app.getUserIDFromWallet(c.Context(), wallet)
	if err != nil {
		return err
	}

	counts := UnreadCountByCategory{}
	err = app.pool.QueryRow(c.Context(), sql, pgx.NamedArgs{
		"user_id": userId,
	}).Scan(&counts.Priority, &counts.General, &counts.Uncategorized)
	if err != nil && err != pgx.ErrNoRows {
		return err
	}

	return c.JSON(CommsResponse{
		Data: counts,
		Health: CommsHealth{
			IsHealthy: true,
		},
	})
}
