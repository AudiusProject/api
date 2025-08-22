package api

import (
	"testing"

	"bridgerton.audius.co/database"
	"bridgerton.audius.co/trashid"
	"github.com/stretchr/testify/assert"
)

func TestV1Notifications(t *testing.T) {
	app := emptyTestApp(t)

	fixtures := database.FixtureMap{
		"notification": []map[string]any{
			{
				"id":        1,
				"specifier": "1234",
				"group_id":  "tip_send:user_id:111:signature:eee",
				"type":      "tip_send",
				"user_ids":  []int{1},
				"data":      []byte(`{"amount": 100000000, "tx_signature": "asdf", "sender_user_id": 111, "receiver_user_id": 222}`),
			},
			{
				"id":        2,
				"specifier": "190321",
				"group_id":  "milestone:PLAYLIST_REPOST_COUNT:id:128608:threshold:10",
				"type":      "milestone",
				"user_ids":  []int{1},
				"data":      []byte(`{"type": "PLAYLIST_REPOST_COUNT", "threshold": 10, "playlist_id": 128608} `),
			},
		},
	}

	database.Seed(app.pool.Replicas[0], fixtures)

	status, body := testGet(t, app, "/v1/notifications/"+trashid.MustEncodeHashID(1))
	assert.Equal(t, 200, status)

	jsonAssert(t, body, map[string]any{
		"data.notifications.0.type":                            "tip_send",
		"data.notifications.0.actions.0.specifier":             "D2Wde",
		"data.notifications.0.actions.0.data.amount":           "1000000000000000000",
		"data.notifications.0.actions.0.data.tip_tx_signature": "asdf",
		"data.notifications.0.actions.0.data.sender_user_id":   "D91oD",
		"data.notifications.0.actions.0.data.receiver_user_id": "n0AML",

		"data.notifications.1.type":                       "milestone",
		"data.notifications.1.actions.0.specifier":        "4W2ay",
		"data.notifications.1.actions.0.data.type":        "playlist_repost_count",
		"data.notifications.1.actions.0.data.is_album":    false,
		"data.notifications.1.actions.0.data.playlist_id": "WQ2P9",
	})
}

func TestV1Notifications_NotDeletedTrack(t *testing.T) {
	app := emptyTestApp(t)

	fixtures := database.FixtureMap{
		"tracks": []map[string]any{
			{
				"track_id":  67576,
				"owner_id":  10,
				"is_delete": false,
			},
		},
		"notification": []map[string]any{
			{
				"id":        1,
				"specifier": "67576",
				"group_id":  "create:track:user_id:67576",
				"type":      "create",
				"user_ids":  []int{1},
				"data":      []byte(`{"track_id": 67576}`),
			},
			{
				"id":        2,
				"specifier": "190321",
				"group_id":  "milestone:PLAYLIST_REPOST_COUNT:id:128608:threshold:10",
				"type":      "milestone",
				"user_ids":  []int{1},
				"data":      []byte(`{"type": "PLAYLIST_REPOST_COUNT", "threshold": 10, "playlist_id": 128608} `),
			},
		},
	}

	database.Seed(app.pool.Replicas[0], fixtures)

	status, body := testGet(t, app, "/v1/notifications/"+trashid.MustEncodeHashID(1))
	assert.Equal(t, 200, status)

	jsonAssert(t, body, map[string]any{
		"data.notifications.#":      2,
		"data.notifications.0.type": "milestone",
		"data.notifications.1.type": "create",
	})
}

func TestV1Notifications_DeletedTrack(t *testing.T) {
	app := emptyTestApp(t)

	fixtures := database.FixtureMap{
		"tracks": []map[string]any{
			{
				"track_id":  67576,
				"owner_id":  10,
				"is_delete": true,
			},
		},
		"notification": []map[string]any{
			{
				"id":        1,
				"specifier": "67576",
				"group_id":  "create:track:user_id:67576",
				"type":      "create",
				"user_ids":  []int{1},
				"data":      []byte(`{"track_id": 67576}`),
			},
			{
				"id":        2,
				"specifier": "190321",
				"group_id":  "milestone:PLAYLIST_REPOST_COUNT:id:128608:threshold:10",
				"type":      "milestone",
				"user_ids":  []int{1},
				"data":      []byte(`{"type": "PLAYLIST_REPOST_COUNT", "threshold": 10, "playlist_id": 128608} `),
			},
		},
	}

	database.Seed(app.pool.Replicas[0], fixtures)

	status, body := testGet(t, app, "/v1/notifications/"+trashid.MustEncodeHashID(1))
	assert.Equal(t, 200, status)

	jsonAssert(t, body, map[string]any{
		"data.notifications.#":      1,
		"data.notifications.0.type": "milestone",
	})
}

func TestV1Notifications_DeletedPlaylist(t *testing.T) {
	app := emptyTestApp(t)

	fixtures := database.FixtureMap{
		"playlists": []map[string]any{
			{
				"playlist_id":       67576,
				"playlist_owner_id": 10,
				"is_delete":         true,
			},
		},
		"notification": []map[string]any{
			{
				"id":        1,
				"specifier": "67576",
				"group_id":  "create:playlist:user_id:67576",
				"type":      "create",
				"user_ids":  []int{1},
				"data":      []byte(`{"playlist_id": 67576}`),
			},
			{
				"id":        2,
				"specifier": "190321",
				"group_id":  "milestone:PLAYLIST_REPOST_COUNT:id:128608:threshold:10",
				"type":      "milestone",
				"user_ids":  []int{1},
				"data":      []byte(`{"type": "PLAYLIST_REPOST_COUNT", "threshold": 10, "playlist_id": 128608} `),
			},
		},
	}

	database.Seed(app.pool.Replicas[0], fixtures)

	status, body := testGet(t, app, "/v1/notifications/"+trashid.MustEncodeHashID(1))
	assert.Equal(t, 200, status)

	jsonAssert(t, body, map[string]any{
		"data.notifications.#":      1,
		"data.notifications.0.type": "milestone",
	})
}

func TestV1Notifications_Comment(t *testing.T) {
	app := emptyTestApp(t)

	fixtures := database.FixtureMap{
		"notification": []map[string]any{
			{
				"id":        1,
				"specifier": "67576",
				"group_id":  "comment:track:user_id:67576:comment_id:1",
				"type":      "comment",
				"user_ids":  []int{1},
				"data":      []byte(`{"comment_id": 1, "type": "Track"}`),
			},
		},
	}

	database.Seed(app.pool.Replicas[0], fixtures)

	status, body := testGet(t, app, "/v1/notifications/"+trashid.MustEncodeHashID(1))
	assert.Equal(t, 200, status)

	jsonAssert(t, body, map[string]any{
		"data.notifications.#":                     1,
		"data.notifications.0.type":                "comment",
		"data.notifications.0.actions.0.data.type": "Track",
	})
}
