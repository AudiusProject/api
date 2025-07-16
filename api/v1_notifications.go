package api

import (
	"bridgerton.audius.co/api/dbv1"
	"bridgerton.audius.co/trashid"
	"github.com/gofiber/fiber/v2"
)

func (app *ApiServer) v1Notifications(c *fiber.Ctx) error {
	notifs, err := app.queries.GetNotifs(c.Context(), dbv1.GetNotifsParams{
		UserID: int32(c.Locals("userId").(int)),
		Lim:    int32(c.QueryInt("limit", 10)),
	})
	if err != nil {
		return err
	}

	// trashify!
	unreadCount := 0
	for idx, notif := range notifs {
		notif.Actions = trashid.HashifyJson(notif.Actions)
		notifs[idx] = notif
		if !notif.IsSeen {
			unreadCount++
		}
	}

	return c.JSON(fiber.Map{
		"data": fiber.Map{
			"notifications": notifs,
			"unread_count":  unreadCount,
		},
	})

}
