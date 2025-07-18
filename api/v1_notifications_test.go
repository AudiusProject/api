package api

import (
	"testing"

	"bridgerton.audius.co/database"
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

	database.Seed(app.pool, fixtures)

	status, body := testGet(t, app, "/v1/notifications/1")
	assert.Equal(t, 200, status)

	jsonAssert(t, body, map[string]any{
		"data.notifications.0.type":                            "tip_send",
		"data.notifications.0.actions.0.data.amount":           "1000000000000000000",
		"data.notifications.0.actions.0.data.tip_tx_signature": "asdf",

		"data.notifications.1.actions.0.data.type":     "playlist_repost_count",
		"data.notifications.1.actions.0.data.is_album": false,
	})

}
