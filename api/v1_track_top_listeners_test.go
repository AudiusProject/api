package api

import (
	"testing"

	"bridgerton.audius.co/database"
	"bridgerton.audius.co/trashid"
	"github.com/stretchr/testify/assert"
)

func TestTrackTopListeners(t *testing.T) {
	app := emptyTestApp(t)

	fixtures := database.FixtureMap{
		"users": []map[string]any{
			{
				"user_id": 1,
				"handle":  "user1",
				"name":    "User 1",
			},
			{
				"user_id": 2,
				"handle":  "user2",
				"name":    "User 2",
			},
			{
				"user_id": 3,
				"handle":  "user3",
				"name":    "User 3",
			},
		},
		"follows": []map[string]any{
			{
				"follower_user_id": 1,
				"followee_user_id": 2,
			},
			{
				"follower_user_id": 1,
				"followee_user_id": 3,
			},
			{
				"follower_user_id": 2,
				"followee_user_id": 3,
			},
		},
		"tracks": []map[string]any{
			{
				"track_id": 1,
				"title":    "Track 1",
				"owner_id": 1,
			},
		},
		"plays": []map[string]any{
			{
				"id":           1,
				"user_id":      1,
				"play_item_id": 1,
				"created_at":   parseTime(t, "2024-01-01"),
			},
			{
				"id":           2,
				"user_id":      1,
				"play_item_id": 1,
				"created_at":   parseTime(t, "2024-01-02"),
			},
			{
				"id":           3,
				"user_id":      2,
				"play_item_id": 1,
				"created_at":   parseTime(t, "2024-01-03"),
			},
			{
				"id":           4,
				"user_id":      3,
				"play_item_id": 1,
				"created_at":   parseTime(t, "2024-01-04"),
			},
			{
				"id":           5,
				"user_id":      3,
				"play_item_id": 1,
				"created_at":   parseTime(t, "2024-01-05"),
			},
		},
	}

	database.Seed(app.pool, fixtures)

	var resp struct {
		Data []FullUserWithPlayCount
	}

	status, body := testGet(t, app, "/v1/full/tracks/"+trashid.MustEncodeHashID(1)+"/top_listeners", &resp)
	assert.Equal(t, 200, status)

	// User 3 and 1 have equal play counts, but user 3 has more followers so should be returned first
	jsonAssert(t, body, map[string]any{
		"data.#":         3,
		"data.0.count":   2,
		"data.0.user.id": trashid.MustEncodeHashID(3),
		"data.1.count":   2,
		"data.1.user.id": trashid.MustEncodeHashID(1),
		"data.2.count":   1,
		"data.2.user.id": trashid.MustEncodeHashID(2),
	})

	var minResp struct {
		Data []MinUserWithPlayCount
	}

	status, body = testGet(t, app, "/v1/tracks/"+trashid.MustEncodeHashID(1)+"/top_listeners", &minResp)
	assert.Equal(t, 200, status)

	jsonAssert(t, body, map[string]any{
		"data.#":         3,
		"data.0.count":   2,
		"data.0.user.id": trashid.MustEncodeHashID(3),
		"data.1.count":   2,
		"data.1.user.id": trashid.MustEncodeHashID(1),
		"data.2.count":   1,
		"data.2.user.id": trashid.MustEncodeHashID(2),
	})
}

func TestTrackTopListenersInvalidParams(t *testing.T) {
	app := emptyTestApp(t)

	baseUrl := "/v1/full/tracks/" + trashid.MustEncodeHashID(10) + "/top_listeners"

	status, _ := testGet(t, app, baseUrl+"?limit=invalid")
	assert.Equal(t, 400, status)

	status, _ = testGet(t, app, baseUrl+"?limit=-1")
	assert.Equal(t, 400, status)

	status, _ = testGet(t, app, baseUrl+"?limit=101")
	assert.Equal(t, 400, status)

	status, _ = testGet(t, app, baseUrl+"?offset=-1")
	assert.Equal(t, 400, status)

	status, _ = testGet(t, app, baseUrl+"?offset=invalid")
	assert.Equal(t, 400, status)
}
