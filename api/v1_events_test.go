package api

import (
	"context"
	"testing"

	"api.audius.co/api/dbv1"
	"api.audius.co/database"
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
		"data.0.event_id":   trashid.MustEncodeHashID(1),
		"data.0.entity_id":  trashid.MustEncodeHashID(100),
		"data.0.permalink":  "/eventsuser/contest/summer-remix-contest",

		"data.1.event_id":   trashid.MustEncodeHashID(2),
		"data.1.entity_id":  trashid.MustEncodeHashID(100),
		"data.1.permalink":  "/eventsuser/contest/live-at-the-venue",

		"data.2.event_id":   trashid.MustEncodeHashID(4),
		"data.2.entity_id":  trashid.MustEncodeHashID(101),
		"data.2.permalink":  "/eventsuser/contest/fall-remix-contest",

		"data.3.event_id":   trashid.MustEncodeHashID(5),
		"data.3.entity_id":  trashid.MustEncodeHashID(101),
		"data.3.permalink":  "/eventsuser/contest/live-fall-show",

		"data.4.event_id":   trashid.MustEncodeHashID(6),
		"data.4.entity_id":  trashid.MustEncodeHashID(102),
		"data.4.permalink":  "/eventsuser2/contest/indie-remix-contest",
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

// TestGetEntityEventsEntryCounts verifies the /events/entity endpoint returns
// related.entry_counts using the same in-window filter as the remix-contests
// discovery endpoint, so callers can prime useRemixesCount({ isContestEntry:
// true }) instead of issuing a separate /tracks/{id}/remixes?limit=0 per card.
func TestGetEntityEventsEntryCounts(t *testing.T) {
	app := emptyTestApp(t)

	hostID := 7101
	remixer := 7102

	contestTrackID := 7001
	contestStart := parseTime(t, "2024-01-02")
	contestEnd := parseTime(t, "2099-01-01")

	inWindow := parseTime(t, "2024-01-03")
	tooEarly := parseTime(t, "2024-01-01") // before contest start => excluded

	fixtures := database.FixtureMap{
		"events": []map[string]any{
			{
				"event_id":    601,
				"event_type":  "remix_contest",
				"entity_type": "track",
				"entity_id":   contestTrackID,
				"user_id":     hostID,
				"created_at":  contestStart,
				"end_date":    contestEnd,
			},
		},
		"users": []map[string]any{
			{"user_id": hostID, "handle": "entryhost"},
			{"user_id": remixer, "handle": "entryremixer"},
		},
		"tracks": []map[string]any{
			{
				"track_id":   contestTrackID,
				"owner_id":   hostID,
				"title":      "Contest Parent",
				"created_at": contestStart,
			},
			{
				"track_id":   7201,
				"owner_id":   remixer,
				"title":      "In Window Entry A",
				"created_at": inWindow,
			},
			{
				"track_id":   7202,
				"owner_id":   remixer,
				"title":      "In Window Entry B",
				"created_at": inWindow,
			},
			{
				"track_id":   7203,
				"owner_id":   remixer,
				"title":      "Too Early (excluded)",
				"created_at": tooEarly,
			},
		},
		"remixes": []map[string]any{
			{"parent_track_id": contestTrackID, "child_track_id": 7201},
			{"parent_track_id": contestTrackID, "child_track_id": 7202},
			{"parent_track_id": contestTrackID, "child_track_id": 7203},
		},
	}
	database.Seed(app.pool.Replicas[0], fixtures)

	contestTrackHash := trashid.MustEncodeHashID(contestTrackID)

	status, body := testGet(
		t, app,
		"/v1/events/entity?entity_id="+contestTrackHash,
	)
	assert.Equal(t, 200, status)

	// 2 in-window remixes counted; the pre-window remix (7203) excluded.
	jsonAssert(t, body, map[string]any{
		"data.0.event_id":  trashid.MustEncodeHashID(601),
		"data.0.entity_id": contestTrackHash,
		"related.entry_counts." + contestTrackHash: float64(2),
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
