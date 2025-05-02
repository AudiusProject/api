package dbv1

import (
	"context"

	"bridgerton.audius.co/trashid"
)

type FullEvent struct {
	GetEventsRow

	EventId  trashid.HashId `json:"event_id"`
	UserId   trashid.HashId `json:"user_id"`
	EntityId trashid.HashId `json:"entity_id"`
}

func (q *Queries) FullEventsKeyed(ctx context.Context, arg GetEventsParams) (map[int32]FullEvent, error) {
	events, err := q.GetEvents(ctx, arg)
	if err != nil {
		return nil, err
	}
	eventMap := map[int32]FullEvent{}
	for _, event := range events {
		eventMap[int32(event.EventID)] = FullEvent{
			GetEventsRow: event,
			EventId:      trashid.HashId(event.EventID),
			UserId:       trashid.HashId(event.UserID),
			EntityId:     trashid.HashId(event.EntityID.Int32),
		}
	}

	return eventMap, nil
}

func (q *Queries) ToFullEvent(event GetEventsRow) FullEvent {
	return FullEvent{
		GetEventsRow: event,
		EventId:      trashid.HashId(event.EventID),
		UserId:       trashid.HashId(event.UserID),
		EntityId:     trashid.HashId(event.EntityID.Int32),
	}
}
