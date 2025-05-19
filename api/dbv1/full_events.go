package dbv1

import (
	"context"
	"time"

	"bridgerton.audius.co/trashid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

type GetEventsParams struct {
	EntityIds     []int32 `json:"entity_ids"`
	EntityType    string  `json:"entity_type"`
	EventType     string  `json:"event_type"`
	FilterDeleted bool    `json:"filter_deleted"`
	OffsetVal     int32   `json:"offset_val"`
	LimitVal      int32   `json:"limit_val"`
}

type FullEvent struct {
	EntityType EventEntityType `json:"entity_type"`
	EventType  EventType       `json:"event_type"`
	EndDate    *time.Time      `json:"end_date"`
	IsDeleted  pgtype.Bool     `json:"is_deleted"`
	CreatedAt  time.Time       `json:"created_at"`
	UpdatedAt  time.Time       `json:"updated_at"`
	EventData  *EventData      `json:"event_data"`

	EventId  trashid.HashId `json:"event_id"`
	UserId   trashid.HashId `json:"user_id"`
	EntityId trashid.HashId `json:"entity_id"`
}

func (q *Queries) FullEvents(ctx context.Context, arg GetEventsParams) ([]FullEvent, error) {

	rows, err := q.db.Query(ctx, mustGetQuery("get_events.sql"), toNamedArgs(arg))
	if err != nil {
		return nil, err
	}

	events, err := pgx.CollectRows(rows, pgx.RowToStructByNameLax[FullEvent])
	if err != nil {
		return nil, err
	}

	// eventMap := map[int32]FullEvent{}
	// for _, event := range events {
	// 	eventMap[int32(event.EventId)] = event
	// }

	return events, nil
}
