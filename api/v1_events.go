package api

import (
	"bridgerton.audius.co/api/dbv1"
	"bridgerton.audius.co/trashid"
	"github.com/gofiber/fiber/v2"
)

type GetEventsParams struct {
	EventType     string `query:"event_type" default:""`
	EntityType    string `query:"entity_type" default:""`
	Limit         int    `query:"limit" default:"25" validate:"min=1,max=100"`
	Offset        int    `query:"offset" default:"0" validate:"min=0"`
	FilterDeleted bool   `query:"filter_deleted" default:"true"`
}

func (app *ApiServer) v1Events(c *fiber.Ctx) error {
	params := GetEventsParams{}
	if err := app.ParseAndValidateQueryParams(c, &params); err != nil {
		return err
	}

	entityIDs := queryMutli(c, "entity_id")
	eventIDs := queryMutli(c, "id")

	entityIds := []int32{}
	for _, id := range entityIDs {
		if id, err := trashid.DecodeHashId(id); err == nil {
			entityIds = append(entityIds, int32(id))
		}
	}

	eventIds := []int32{}
	for _, id := range eventIDs {
		if id, err := trashid.DecodeHashId(id); err == nil {
			eventIds = append(eventIds, int32(id))
		}
	}
	recentEvents, err := app.queries.GetEvents(c.Context(), dbv1.GetEventsParams{
		EntityIds:     entityIds,
		EventIds:      eventIds,
		EventType:     params.EventType,
		EntityType:    params.EntityType,
		LimitVal:      int32(params.Limit),
		OffsetVal:     int32(params.Offset),
		FilterDeleted: !params.FilterDeleted,
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
