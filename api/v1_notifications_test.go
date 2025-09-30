package api

import (
	"testing"

	"api.audius.co/database"
	"api.audius.co/trashid"
	"github.com/stretchr/testify/assert"
)

func TestV1Notifications(t *testing.T) {
	app := emptyTestApp(t)

	fixtures := database.FixtureMap{
		"notification": []map[string]any{
			{
				"id":        1,
				"specifier": "111",
				"group_id":  "tip_send:user_id:111:signature:eee",
				"type":      "tip_send",
				"user_ids":  []int{1},
				"data":      []byte(`{"amount": 100000000, "tx_signature": "asdf", "sender_user_id": 111, "receiver_user_id": 222}`),
			},
			{
				"id":        2,
				"specifier": "128608",
				"group_id":  "milestone:PLAYLIST_REPOST_COUNT:id:128608:threshold:10",
				"type":      "milestone",
				"user_ids":  []int{1},
				"data":      []byte(`{"type": "PLAYLIST_REPOST_COUNT", "threshold": 10, "playlist_id": 128608} `),
			},
			{
				"id":        3,
				"specifier": "100",
				"group_id":  "usdc_purchase_seller:seller_user_id:1:buyer_user_id:100:content_id:1118440:content_type:track",
				"type":      "usdc_purchase_seller",
				"user_ids":  []int{1},
				"data":      []byte(`{"amount": 3000000, "vendor": "user_bank", "content_id": 1118440, "content_type": "track", "extra_amount": 0, "buyer_user_id": 100, "seller_user_id": 1}`),
			},
		},
	}

	database.Seed(app.pool.Replicas[0], fixtures)

	status, body := testGet(t, app, "/v1/notifications/"+trashid.MustEncodeHashID(1))
	assert.Equal(t, 200, status)

	jsonAssert(t, body, map[string]any{
		"data.notifications.0.type":                          "usdc_purchase_seller",
		"data.notifications.0.actions.0.specifier":           trashid.MustEncodeHashID(100),
		"data.notifications.0.actions.0.data.amount":         "3000000",
		"data.notifications.0.actions.0.data.vendor":         "user_bank",
		"data.notifications.0.actions.0.data.content_id":     trashid.MustEncodeHashID(1118440),
		"data.notifications.0.actions.0.data.content_type":   "track",
		"data.notifications.0.actions.0.data.extra_amount":   "0",
		"data.notifications.0.actions.0.data.buyer_user_id":  trashid.MustEncodeHashID(100),
		"data.notifications.0.actions.0.data.seller_user_id": trashid.MustEncodeHashID(1),

		"data.notifications.1.type":                            "tip_send",
		"data.notifications.1.actions.0.specifier":             trashid.MustEncodeHashID(111),
		"data.notifications.1.actions.0.data.amount":           "1000000000000000000",
		"data.notifications.1.actions.0.data.tip_tx_signature": "asdf",
		"data.notifications.1.actions.0.data.sender_user_id":   trashid.MustEncodeHashID(111),
		"data.notifications.1.actions.0.data.receiver_user_id": trashid.MustEncodeHashID(222),

		"data.notifications.2.type":                       "milestone",
		"data.notifications.2.actions.0.specifier":        trashid.MustEncodeHashID(128608),
		"data.notifications.2.actions.0.data.type":        "playlist_repost_count",
		"data.notifications.2.actions.0.data.is_album":    false,
		"data.notifications.2.actions.0.data.playlist_id": trashid.MustEncodeHashID(128608),
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

func TestV1Notifications_DeactivatedUser(t *testing.T) {
	app := emptyTestApp(t)

	fixtures := database.FixtureMap{
		"users": []map[string]any{
			{
				"user_id":        67576,
				"is_deactivated": true,
			},
			{
				"user_id":        1235,
				"is_deactivated": false,
			},
		},
		"notification": []map[string]any{
			{ // Deactivated user
				"id":        1,
				"specifier": "1234",
				"group_id":  "comment:track:user_id:67576:comment_id:1",
				"type":      "comment",
				"user_ids":  []int{1},
				"data":      []byte(`{"comment_id": 1, "type": "Track", "entity_user_id": 67576}`),
			},
			{
				"id":        2,
				"specifier": "1235",
				"group_id":  "comment:track:user_id:1235:comment_id:1",
				"type":      "comment",
				"user_ids":  []int{1},
				"data":      []byte(`{"comment_id": 1, "type": "Track", "entity_user_id": 1235}`),
			},
		},
	}

	database.Seed(app.pool.Replicas[0], fixtures)

	status, body := testGet(t, app, "/v1/notifications/"+trashid.MustEncodeHashID(1))
	assert.Equal(t, 200, status)

	jsonAssert(t, body, map[string]any{
		"data.notifications.#":                     1,
		"data.notifications.0.type":                "comment",
		"data.notifications.0.actions.0.specifier": trashid.MustEncodeHashID(1235),
	})
}

func TestV1Notifications_LowScore(t *testing.T) {
	app := emptyTestApp(t)

	fixtures := database.FixtureMap{
		"users": []map[string]any{
			{
				"user_id":        67576,
				"is_deactivated": false,
			},
			{
				"user_id":        1235,
				"is_deactivated": false,
			},
		},
		"aggregate_user": []map[string]any{
			{
				"user_id": 67576,
				"score":   -1,
			},
			{
				"user_id": 1235,
				"score":   0,
			},
		},
		"notification": []map[string]any{
			{ // Low score
				"id":        1,
				"specifier": "67576",
				"group_id":  "comment:track:user_id:67576:comment_id:1",
				"type":      "comment",
				"user_ids":  []int{1},
				"data":      []byte(`{"comment_id": 1, "type": "Track", "entity_user_id": 67576}`),
			},
			{
				"id":        2,
				"specifier": "1235",
				"group_id":  "comment:track:user_id:1235:comment_id:1",
				"type":      "comment",
				"user_ids":  []int{1},
				"data":      []byte(`{"comment_id": 1, "type": "Track", "entity_user_id": 1235}`),
			},
		},
	}

	database.Seed(app.pool.Replicas[0], fixtures)

	status, body := testGet(t, app, "/v1/notifications/"+trashid.MustEncodeHashID(1))
	assert.Equal(t, 200, status)

	jsonAssert(t, body, map[string]any{
		"data.notifications.#":                     1,
		"data.notifications.0.type":                "comment",
		"data.notifications.0.actions.0.specifier": trashid.MustEncodeHashID(1235),
	})
}

func TestV1Notifications_UnlistedTrack(t *testing.T) {
	app := emptyTestApp(t)

	fixtures := database.FixtureMap{
		"users": []map[string]any{
			{
				"user_id":        67576,
				"is_deactivated": true,
			},
		},
		"tracks": []map[string]any{
			{
				"track_id":    1,
				"owner_id":    10,
				"is_unlisted": false,
			},
			{
				"track_id":    2,
				"owner_id":    10,
				"is_unlisted": true,
			},
		},
		"notification": []map[string]any{
			{
				"id":        1,
				"specifier": trashid.MustEncodeHashID(1),
				"group_id":  "create:track:user_id:10",
				"type":      "create",
				"user_ids":  []int{1},
				"data":      []byte(`{"track_id": 1}`),
			},
			{
				"id":        2,
				"specifier": trashid.MustEncodeHashID(2),
				"group_id":  "create:track:user_id:10",
				"type":      "create",
				"user_ids":  []int{1},
				"data":      []byte(`{"track_id": 2}`),
			},
		},
	}

	database.Seed(app.pool.Replicas[0], fixtures)

	status, body := testGet(t, app, "/v1/notifications/"+trashid.MustEncodeHashID(1))
	assert.Equal(t, 200, status)

	jsonAssert(t, body, map[string]any{
		"data.notifications.#":                     1,
		"data.notifications.0.type":                "create",
		"data.notifications.0.actions.0.specifier": trashid.MustEncodeHashID(1),
	})
}

func TestV1Notifications_PrivatePlaylist(t *testing.T) {
	app := emptyTestApp(t)

	fixtures := database.FixtureMap{
		"playlists": []map[string]any{
			{
				"playlist_id":       67576,
				"playlist_owner_id": 10,
				"is_private":        true,
			},
		},
		"notification": []map[string]any{
			{
				"id":        1,
				"specifier": trashid.MustEncodeHashID(67576),
				"group_id":  "create:playlist:user_id:67576",
				"type":      "create",
				"user_ids":  []int{1},
				"data":      []byte(`{"playlist_id": 67576}`),
			},
			{
				"id":        2,
				"specifier": trashid.MustEncodeHashID(67576),
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
