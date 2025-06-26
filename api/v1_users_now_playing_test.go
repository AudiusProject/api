package api

import (
	"testing"
	"time"

	"bridgerton.audius.co/trashid"
	"github.com/stretchr/testify/assert"
)

func TestUsersNowPlayingActive(t *testing.T) {
	app := emptyTestApp(t)

	createFixtures(app, FixtureMap{
		"users": []map[string]any{
			{
				"user_id": 1,
				"handle":  "tester",
			},
		},
		"tracks": []map[string]any{
			{
				"track_id": 9001,
				"owner_id": 1,
				"title":    "Test Track",
				"duration": 120,
			},
		},
		"plays": []map[string]any{
			{
				"id":           1,
				"user_id":      1,
				"play_item_id": 9001,
				"created_at":   time.Now().UTC().Add(-30 * time.Second),
			},
		},
	})

	path := "/v1/users/" + trashid.MustEncodeHashID(1) + "/now-playing"
	status, body := testGet(t, app, path)
	assert.Equal(t, 200, status)
	jsonAssert(t, body, map[string]any{
		"data.title": "Test Track",
		"data.id":    "9001",
	})
}

func TestUsersNowPlayingInactive(t *testing.T) {
	app := emptyTestApp(t)

	createFixtures(app, FixtureMap{
		"users": []map[string]any{
			{
				"user_id": 2,
				"handle":  "tester",
			},
		},
		"tracks": []map[string]any{
			{
				"track_id": 9002,
				"owner_id": 2,
				"title":    "Test Track",
				"duration": 60,
			},
		},
		"plays": []map[string]any{
			{
				"id":           1,
				"user_id":      2,
				"play_item_id": 9002,
				"created_at":   time.Now().UTC().Add(-2 * time.Minute),
			},
		},
	})

	path := "/v1/users/" + trashid.MustEncodeHashID(2) + "/now-playing"
	status, body := testGet(t, app, path)
	assert.Equal(t, 200, status)
	jsonAssert(t, body, map[string]any{
		"data": nil,
	})
}

func TestUsersNowPlayingNoPlays(t *testing.T) {
	app := emptyTestApp(t)

	// create user & track but no play
	createFixtures(app, FixtureMap{
		"users": []map[string]any{
			{"user_id": 3, "handle": "tester"},
		},
		"tracks": []map[string]any{
			{"track_id": 9003, "owner_id": 3, "title": "Test Track", "duration": 180},
		},
	})

	path := "/v1/users/" + trashid.MustEncodeHashID(int(3)) + "/now-playing"
	status, body := testGet(t, app, path)
	assert.Equal(t, 200, status)
	jsonAssert(t, body, map[string]any{
		"data": nil,
	})
}
