package api

import (
	"api.audius.co/api/dbv1"
	"api.audius.co/trashid"
	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"
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

	entityIDs := queryMulti(c, "entity_id")
	eventIDs := queryMulti(c, "id")

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

	// Compute per-contest entry counts so consumers that resolve a contest via
	// this endpoint (the track-page contest section, cold/deep-linked contest
	// pages, web Explore's featured contests) can prime
	// useRemixesCount({ isContestEntry: true }) instead of firing a separate
	// /tracks/{id}/remixes?only_contest_entries=true&limit=0 per card. Mirrors
	// the entry-count filter in v1EventsRemixContests: a child track is an entry
	// iff it was created after the contest started, before its end_date, and is
	// currently listed. Only remix_contest events on track entities have a
	// meaningful entry count.
	entryCounts := map[string]int64{}
	contestEventIds := []int32{}
	for _, event := range recentEvents {
		if event.EventType == dbv1.EventTypeRemixContest &&
			event.EntityType == dbv1.EventEntityTypeTrack &&
			event.EntityID.Valid {
			contestEventIds = append(contestEventIds, event.EventID)
			// Default to 0 so the UI primes a definitive "no entries" and
			// still skips the count-only request for empty contests.
			entryCounts[trashid.MustEncodeHashID(int(event.EntityID.Int32))] = 0
		}
	}

	if len(contestEventIds) > 0 {
		countSql := `
			SELECT e.entity_id, COUNT(DISTINCT ct.track_id) AS entry_count
			FROM events e
			JOIN remixes rm ON rm.parent_track_id = e.entity_id
			JOIN tracks ct ON ct.track_id = rm.child_track_id
			WHERE e.event_id = ANY(@event_ids)
				AND ct.is_current = true
				AND ct.is_delete = false
				AND ct.is_unlisted = false
				AND ct.created_at > e.created_at
				AND (e.end_date IS NULL OR ct.created_at < e.end_date)
			GROUP BY e.entity_id;
		`
		rows, err := app.pool.Query(c.Context(), countSql, pgx.NamedArgs{
			"event_ids": contestEventIds,
		})
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var entityID int32
			var entryCount int64
			if err := rows.Scan(&entityID, &entryCount); err != nil {
				return err
			}
			entryCounts[trashid.MustEncodeHashID(int(entityID))] = entryCount
		}
		if err := rows.Err(); err != nil {
			return err
		}
	}

	return c.JSON(fiber.Map{
		"data": data,
		"related": fiber.Map{
			"entry_counts": entryCounts,
		},
	})
}
