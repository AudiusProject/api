package api

import (
	"testing"

	"bridgerton.audius.co/database"
	"bridgerton.audius.co/trashid"
	"github.com/stretchr/testify/assert"
)

func TestV1UsersSupporting(t *testing.T) {
	app := emptyTestApp(t)

	fixtures := database.FixtureMap{
		"users": {
			{"user_id": 1, "handle": "supported"},
			{"user_id": 2, "handle": "supporting_1"},
			{"user_id": 3, "handle": "supporting_2"},
			{"user_id": 4, "handle": "supporting_3"},
			{"user_id": 5, "handle": "supporting_4"},
		},
		"aggregate_user_tips": {
			{"receiver_user_id": 2, "sender_user_id": 1, "amount": 1000},
			{"receiver_user_id": 2, "sender_user_id": 3, "amount": 5000},
			{"receiver_user_id": 2, "sender_user_id": 4, "amount": 1000},
			{"receiver_user_id": 2, "sender_user_id": 5, "amount": 3000},

			{"receiver_user_id": 3, "sender_user_id": 1, "amount": 3000},
			{"receiver_user_id": 3, "sender_user_id": 2, "amount": 1000},
			{"receiver_user_id": 3, "sender_user_id": 4, "amount": 5000},
			{"receiver_user_id": 3, "sender_user_id": 5, "amount": 2000},

			{"receiver_user_id": 4, "sender_user_id": 1, "amount": 2500},
			{"receiver_user_id": 4, "sender_user_id": 2, "amount": 1000},
			{"receiver_user_id": 4, "sender_user_id": 3, "amount": 1000},
			{"receiver_user_id": 4, "sender_user_id": 5, "amount": 3000},

			{"receiver_user_id": 5, "sender_user_id": 1, "amount": 2000},
			{"receiver_user_id": 5, "sender_user_id": 2, "amount": 1000},
			{"receiver_user_id": 5, "sender_user_id": 3, "amount": 5000},
			{"receiver_user_id": 5, "sender_user_id": 4, "amount": 5000},
		},
	}

	database.Seed(app.pool.Replicas[0], fixtures)

	{
		status, body := testGet(t, app, "/v1/users/7eP5n/supporting")
		assert.Equal(t, 200, status)
		jsonAssert(t, body, map[string]any{"data.0.receiver.id": trashid.MustEncodeHashID(3)})
		jsonAssert(t, body, map[string]any{"data.0.rank": 2})
		jsonAssert(t, body, map[string]any{"data.1.receiver.id": trashid.MustEncodeHashID(4)})
		jsonAssert(t, body, map[string]any{"data.1.rank": 2})
		jsonAssert(t, body, map[string]any{"data.2.receiver.id": trashid.MustEncodeHashID(5)})
		jsonAssert(t, body, map[string]any{"data.2.rank": 3})
		jsonAssert(t, body, map[string]any{"data.3.receiver.id": trashid.MustEncodeHashID(2)})
		jsonAssert(t, body, map[string]any{"data.3.rank": 3})
	}

	// just user_id 2
	{
		status, body := testGet(t, app, "/v1/users/7eP5n/supporting/ML51L")
		assert.Equal(t, 200, status)
		jsonAssert(t, body, map[string]any{"data.receiver.id": trashid.MustEncodeHashID(2)})
		jsonAssert(t, body, map[string]any{"data.rank": 3})
	}
}
