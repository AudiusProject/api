package api

import (
	"context"
	"testing"

	"api.audius.co/api/dbv1"
	"api.audius.co/trashid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

func TestGetEventsExcludesDeletedTracks(t *testing.T) {
	app := testAppWithFixtures(t)
	ctx := context.Background()
	require.NotNil(t, app.writePool, "test requires write pool")

	// Mark track 102 (which has event 6) as deleted
	_, err := app.writePool.Exec(ctx, `UPDATE tracks SET is_delete = true WHERE track_id = 102 AND is_current = true`)
	require.NoError(t, err)

	var eventsResponse struct {
		Data []dbv1.FullEvent
	}
	status, body := testGet(t, app, "/v1/events", &eventsResponse)
	assert.Equal(t, 200, status)

	// Event 6 is for entity_id 102; it must not appear when the track is deleted
	entity102Hash := trashid.MustEncodeHashID(102)
	for _, e := range eventsResponse.Data {
		assert.NotEqual(t, entity102Hash, e.EntityId, "events for deleted track 102 must not be returned")
	}

	// Should have 4 events (1,2,4,5), not 5 (event 6 excluded)
	assert.Len(t, eventsResponse.Data, 4, "expected 4 events after excluding event for deleted track")
	jsonAssert(t, body, map[string]any{
		"data.0.event_id":  trashid.MustEncodeHashID(1),
		"data.0.entity_id": trashid.MustEncodeHashID(100),
		"data.3.event_id":  trashid.MustEncodeHashID(5),
		"data.3.entity_id": trashid.MustEncodeHashID(101),
	})
}

func TestGetEventsExcludesAccessAuthoritiesTracks(t *testing.T) {
	app := testAppWithFixtures(t)
	ctx := context.Background()
	require.NotNil(t, app.writePool, "test requires write pool")

	// Set access_authorities on track 102 (which has event 6) so it is gated
	_, err := app.writePool.Exec(ctx, `UPDATE tracks SET access_authorities = ARRAY['0x123']::text[] WHERE track_id = 102 AND is_current = true`)
	require.NoError(t, err)

	var eventsResponse struct {
		Data []dbv1.FullEvent
	}
	status, _ := testGet(t, app, "/v1/events", &eventsResponse)
	assert.Equal(t, 200, status)

	// Event 6 is for entity_id 102; it must not appear when the track has access_authorities
	entity102Hash := trashid.MustEncodeHashID(102)
	for _, e := range eventsResponse.Data {
		assert.NotEqual(t, entity102Hash, e.EntityId, "events for access_authorities track 102 must not be returned")
	}

	assert.Len(t, eventsResponse.Data, 4, "expected 4 events after excluding event for access_authorities track")
}
