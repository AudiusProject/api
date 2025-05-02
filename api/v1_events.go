package api

import (
	"bridgerton.audius.co/api/dbv1"
	"bridgerton.audius.co/trashid"
	"github.com/gofiber/fiber/v2"
)

func (app *ApiServer) v1Events(c *fiber.Ctx) error {
	limit := c.QueryInt("limit", 25)
	offset := c.QueryInt("offset", 0)
	eventType := c.Query("event_type", "")
	entityType := c.Query("entity_type", "")
	filterDeleted := c.QueryBool("filter_deleted", true)
	entityIDs := queryMutli(c, "entity_id")

	entityIds := []int32{}
	for _, id := range entityIDs {
		if id, err := trashid.DecodeHashId(id); err == nil {
			entityIds = append(entityIds, int32(id))
		}
	}

	recentEvents, err := app.queries.GetEvents(c.Context(), dbv1.GetEventsParams{
		EntityIds:     entityIds,
		EventType:     eventType,
		EntityType:    entityType,
		LimitVal:      int32(limit),
		OffsetVal:     int32(offset),
		FilterDeleted: !filterDeleted,
	})
	if err != nil {
		return err
	}

	data := []dbv1.FullEvent{}
	for _, event := range recentEvents {
		data = append(data, app.queries.ToFullEvent(event))
	}

	return c.JSON(fiber.Map{
		"data": data,
	})
}
