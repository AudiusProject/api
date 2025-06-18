package api

import (
	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"
)

type CommentNotificationSetting struct {
	IsMuted bool `json:"is_muted"`
}

func (app *ApiServer) v1TrackCommentNotificationSetting(c *fiber.Ctx) error {
	myId := app.getMyId(c)
	sql := `
	SELECT is_muted
	FROM comment_notification_settings
	WHERE user_id = @userId
	AND entity_type = 'Track'
	AND entity_id = @trackId
	`

	rows, err := app.pool.Query(c.Context(), sql, pgx.NamedArgs{
		"userId":  myId,
		"trackId": c.Locals("trackId"),
	})
	if err != nil {
		return err
	}

	isMuted, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[CommentNotificationSetting])
	if err != nil {
		if err == pgx.ErrNoRows {
			return c.JSON(fiber.Map{
				"data": CommentNotificationSetting{IsMuted: false},
			})
		}
		return err
	}

	return c.JSON(fiber.Map{
		"data": isMuted,
	})
}
