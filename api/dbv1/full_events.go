package dbv1

import (
	"api.audius.co/trashid"
)

type FullEvent struct {
	GetEventsRow

	EventId  trashid.HashId `json:"event_id"`
	UserId   trashid.HashId `json:"user_id"`
	EntityId trashid.HashId `json:"entity_id"`
}

func (q *Queries) ToFullEvent(event GetEventsRow) FullEvent {
	return FullEvent{
		GetEventsRow: event,
		EventId:      trashid.HashId(event.EventID),
		UserId:       trashid.HashId(event.UserID),
		EntityId:     trashid.HashId(event.EntityID.Int32),
	}
}
