package api

import (
	"testing"

	"api.audius.co/database"
	"api.audius.co/trashid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The event_comments endpoint returns the top-level comment stream attached to
// a remix-contest event. These tests verify the endpoint's shape, filtering,
// and the artist-reacted resolution we added for events.

func testEventCommentsBaseUsers() []map[string]any {
	return []map[string]any{
		{
			"user_id":   1,
			"handle":    "eventartist",
			"handle_lc": "eventartist",
			"name":      "Event Artist",
			"wallet":    "0xe0f1230000000000000000000000000000000001",
		},
		{
			"user_id":   2,
			"handle":    "eventfan",
			"handle_lc": "eventfan",
			"name":      "Event Fan",
			"wallet":    "0xe0f1230000000000000000000000000000000002",
		},
		{
			"user_id":   3,
			"handle":    "eventfan2",
			"handle_lc": "eventfan2",
			"name":      "Event Fan 2",
			"wallet":    "0xe0f1230000000000000000000000000000000003",
		},
	}
}

func TestEventComments_UnknownEvent(t *testing.T) {
	app := emptyTestApp(t)
	enc, err := trashid.EncodeHashId(999999)
	require.NoError(t, err)
	status, _ := testGet(t, app, "/v1/events/"+enc+"/comments")
	assert.Equal(t, 404, status)
}

func TestEventComments_ReturnsTopLevelOnlyNewestFirst(t *testing.T) {
	app := emptyTestApp(t)

	// Event 100 is owned by user 1 and points at track 1.
	// Comments 600 and 601 are both top-level; 602 is a reply to 600 and should
	// NOT appear in the top-level listing (it comes back nested instead).
	database.Seed(app.pool.Replicas[0], database.FixtureMap{
		"users": testEventCommentsBaseUsers(),
		"tracks": {
			{
				"track_id":   1,
				"owner_id":   1,
				"title":      "Original",
				"created_at": "2020-01-01 00:00:00",
			},
		},
		"events": {
			{
				"event_id":    100,
				"event_type":  "remix_contest",
				"user_id":     1,
				"entity_type": "track",
				"entity_id":   1,
				"event_data":  map[string]any{"description": "remix me"},
				"created_at":  "2020-05-01 00:00:00",
				"updated_at":  "2020-05-01 00:00:00",
			},
		},
		"comments": []map[string]any{
			{
				"comment_id":  600,
				"user_id":     2,
				"entity_id":   100,
				"entity_type": "Event",
				"text":        "first!",
				"created_at":  "2020-06-01 00:00:00",
			},
			{
				"comment_id":  601,
				"user_id":     1,
				"entity_id":   100,
				"entity_type": "Event",
				"text":        "thanks for joining",
				"created_at":  "2020-06-02 00:00:00",
			},
			{
				"comment_id":  602,
				"user_id":     1,
				"entity_id":   100,
				"entity_type": "Event",
				"text":        "and welcome",
				"created_at":  "2020-06-03 00:00:00",
			},
		},
		// Comment 602 is threaded under 600
		"comment_threads": []map[string]any{
			{"parent_comment_id": 600, "comment_id": 602},
		},
	})

	encEvent, err := trashid.EncodeHashId(100)
	require.NoError(t, err)
	enc600, err := trashid.EncodeHashId(600)
	require.NoError(t, err)
	enc601, err := trashid.EncodeHashId(601)
	require.NoError(t, err)
	encUser1, err := trashid.EncodeHashId(1)
	require.NoError(t, err)

	status, body := testGet(t, app, "/v1/events/"+encEvent+"/comments?sort_method=newest")
	require.Equal(t, 200, status, string(body))

	// Default sort is newest-first, so 601 (Jun 2) should come before 600 (Jun 1).
	// The reply (602) must NOT appear as its own top-level item.
	jsonAssert(t, body, map[string]any{
		"data.#":           2,
		"data.0.id":        enc601,
		"data.0.message":   "thanks for joining",
		"data.1.id":        enc600,
		"data.1.message":   "first!",
		"event_user_id":    encUser1,
		"data.1.replies.#": 1,
	})
}

func TestEventComments_IsArtistReactedResolvesViaEventOwner(t *testing.T) {
	app := emptyTestApp(t)

	// Event 200 is owned by user 1. User 2 writes a comment. User 1 (the event
	// artist) reacts to it. The is_artist_reacted field should be true even
	// though the comment's entity_id points at an event, not a track — the
	// COALESCE(tracks.owner_id, events.user_id, comments.entity_id) resolution
	// should pick up events.user_id.
	database.Seed(app.pool.Replicas[0], database.FixtureMap{
		"users": testEventCommentsBaseUsers(),
		"tracks": {
			{
				"track_id":   1,
				"owner_id":   1,
				"title":      "Original",
				"created_at": "2020-01-01 00:00:00",
			},
		},
		"events": {
			{
				"event_id":    200,
				"event_type":  "remix_contest",
				"user_id":     1,
				"entity_type": "track",
				"entity_id":   1,
				"event_data":  map[string]any{"description": "remix me"},
				"created_at":  "2020-05-01 00:00:00",
				"updated_at":  "2020-05-01 00:00:00",
			},
		},
		"comments": []map[string]any{
			{
				"comment_id":  700,
				"user_id":     2,
				"entity_id":   200,
				"entity_type": "Event",
				"text":        "here's my remix",
				"created_at":  "2020-06-01 00:00:00",
			},
		},
		"comment_reactions": []map[string]any{
			{
				"comment_id": 700,
				"user_id":    1, // event owner reacted
				"is_delete":  false,
				"created_at": "2020-06-01 01:00:00",
				"updated_at": "2020-06-01 01:00:00",
			},
		},
	})

	encEvent, err := trashid.EncodeHashId(200)
	require.NoError(t, err)

	status, body := testGet(t, app, "/v1/events/"+encEvent+"/comments")
	require.Equal(t, 200, status, string(body))

	jsonAssert(t, body, map[string]any{
		"data.#":                    1,
		"data.0.message":            "here's my remix",
		"data.0.react_count":        1,
		"data.0.is_artist_reacted":  true,
	})
}

func TestEventComments_DeletedEventReturns404(t *testing.T) {
	app := emptyTestApp(t)
	database.Seed(app.pool.Replicas[0], database.FixtureMap{
		"users": testEventCommentsBaseUsers(),
		"tracks": {
			{
				"track_id":   1,
				"owner_id":   1,
				"title":      "Original",
				"created_at": "2020-01-01 00:00:00",
			},
		},
		"events": {
			{
				"event_id":    300,
				"event_type":  "remix_contest",
				"user_id":     1,
				"entity_type": "track",
				"entity_id":   1,
				"event_data":  map[string]any{"description": "remix me"},
				"is_deleted":  true,
				"created_at":  "2020-05-01 00:00:00",
				"updated_at":  "2020-05-01 00:00:00",
			},
		},
	})

	encEvent, err := trashid.EncodeHashId(300)
	require.NoError(t, err)
	status, _ := testGet(t, app, "/v1/events/"+encEvent+"/comments")
	assert.Equal(t, 404, status)
}

func TestEventComments_PaginationAndTimestampSort(t *testing.T) {
	app := emptyTestApp(t)

	// Three top-level comments; with sort_method=timestamp we expect oldest
	// first. offset=1&limit=1 returns the middle one.
	database.Seed(app.pool.Replicas[0], database.FixtureMap{
		"users": testEventCommentsBaseUsers(),
		"tracks": {
			{
				"track_id":   1,
				"owner_id":   1,
				"title":      "Original",
				"created_at": "2020-01-01 00:00:00",
			},
		},
		"events": {
			{
				"event_id":    400,
				"event_type":  "remix_contest",
				"user_id":     1,
				"entity_type": "track",
				"entity_id":   1,
				"event_data":  map[string]any{"description": "remix me"},
				"created_at":  "2020-05-01 00:00:00",
				"updated_at":  "2020-05-01 00:00:00",
			},
		},
		"comments": []map[string]any{
			{
				"comment_id":  800,
				"user_id":     2,
				"entity_id":   400,
				"entity_type": "Event",
				"text":        "oldest",
				"created_at":  "2020-06-01 00:00:00",
			},
			{
				"comment_id":  801,
				"user_id":     3,
				"entity_id":   400,
				"entity_type": "Event",
				"text":        "middle",
				"created_at":  "2020-06-02 00:00:00",
			},
			{
				"comment_id":  802,
				"user_id":     2,
				"entity_id":   400,
				"entity_type": "Event",
				"text":        "newest",
				"created_at":  "2020-06-03 00:00:00",
			},
		},
	})

	encEvent, err := trashid.EncodeHashId(400)
	require.NoError(t, err)

	status, body := testGet(t, app, "/v1/events/"+encEvent+"/comments?sort_method=timestamp&offset=1&limit=1")
	require.Equal(t, 200, status, string(body))
	jsonAssert(t, body, map[string]any{
		"data.#":         1,
		"data.0.message": "middle",
	})
}

func TestEventComments_DoesNotLeakTrackComments(t *testing.T) {
	app := emptyTestApp(t)

	// Track 1 has a comment, event 500 is attached to track 1. The event's
	// endpoint should return nothing (only event-scoped comments), even though
	// there's a track comment with entity_id=1 in the same table.
	database.Seed(app.pool.Replicas[0], database.FixtureMap{
		"users": testEventCommentsBaseUsers(),
		"tracks": {
			{
				"track_id":   1,
				"owner_id":   1,
				"title":      "Original",
				"created_at": "2020-01-01 00:00:00",
			},
		},
		"events": {
			{
				"event_id":    500,
				"event_type":  "remix_contest",
				"user_id":     1,
				"entity_type": "track",
				"entity_id":   1,
				"event_data":  map[string]any{"description": "remix me"},
				"created_at":  "2020-05-01 00:00:00",
				"updated_at":  "2020-05-01 00:00:00",
			},
		},
		"comments": []map[string]any{
			{
				"comment_id":  900,
				"user_id":     2,
				"entity_id":   1, // track comment
				"entity_type": "Track",
				"text":        "great track",
				"created_at":  "2020-06-01 00:00:00",
			},
		},
	})

	encEvent, err := trashid.EncodeHashId(500)
	require.NoError(t, err)
	status, body := testGet(t, app, "/v1/events/"+encEvent+"/comments")
	require.Equal(t, 200, status, string(body))
	jsonAssert(t, body, map[string]any{
		"data.#": 0,
	})
}
