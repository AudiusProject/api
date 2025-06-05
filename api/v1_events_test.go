package api

import (
	"testing"

	"bridgerton.audius.co/api/dbv1"
	"bridgerton.audius.co/trashid"
	"github.com/stretchr/testify/assert"
)

func TestGetEvents(t *testing.T) {
	app := testAppWithFixtures(t)
	var eventsResponse struct {
		Data []dbv1.FullEvent
	}

	status, body := testGet(t, app, "/v1/events", &eventsResponse)
	assert.Equal(t, 200, status)

	jsonAssert(t, body, map[string]any{
		"data.0.event_id":  trashid.MustEncodeHashID(1),
		"data.0.entity_id": trashid.MustEncodeHashID(100),

		"data.1.event_id":  trashid.MustEncodeHashID(2),
		"data.1.entity_id": trashid.MustEncodeHashID(100),

		"data.2.event_id":  trashid.MustEncodeHashID(4),
		"data.2.entity_id": trashid.MustEncodeHashID(101),

		"data.3.event_id":  trashid.MustEncodeHashID(5),
		"data.3.entity_id": trashid.MustEncodeHashID(101),

		"data.4.event_id":  trashid.MustEncodeHashID(6),
		"data.4.entity_id": trashid.MustEncodeHashID(102),
	})
}

func TestGetEventsEntity(t *testing.T) {
	app := testAppWithFixtures(t)
	var eventsResponse struct {
		Data []dbv1.FullEvent
	}

	status, body := testGet(
		t, app,
		"/v1/events/entity?entity_id="+trashid.MustEncodeHashID(102),
		&eventsResponse,
	)
	assert.Equal(t, 200, status)

	jsonAssert(t, body, map[string]any{
		"data.0.event_id":  trashid.MustEncodeHashID(6),
		"data.0.entity_id": trashid.MustEncodeHashID(102),
	})
}
