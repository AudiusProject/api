package api

import (
	"testing"

	"bridgerton.audius.co/database"
	"bridgerton.audius.co/trashid"
	"github.com/stretchr/testify/assert"
)

func TestV1UsersSupporters(t *testing.T) {
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
			{"receiver_user_id": 1, "sender_user_id": 2, "amount": 1000},
			{"receiver_user_id": 1, "sender_user_id": 3, "amount": 3000},
			{"receiver_user_id": 1, "sender_user_id": 4, "amount": 2000},
			{"receiver_user_id": 1, "sender_user_id": 5, "amount": 2000},
		},
	}

	database.Seed(app.pool, fixtures)

	{
		status, body := testGet(t, app, "/v1/users/7eP5n/supporters")
		assert.Equal(t, 200, status)
		jsonAssert(t, body, map[string]any{"data.0.sender.id": trashid.MustEncodeHashID(3)})
		jsonAssert(t, body, map[string]any{"data.0.rank": 1})
		jsonAssert(t, body, map[string]any{"data.1.sender.id": trashid.MustEncodeHashID(4)})
		jsonAssert(t, body, map[string]any{"data.1.rank": 2})
		jsonAssert(t, body, map[string]any{"data.2.sender.id": trashid.MustEncodeHashID(5)})
		jsonAssert(t, body, map[string]any{"data.2.rank": 2})
		jsonAssert(t, body, map[string]any{"data.3.sender.id": trashid.MustEncodeHashID(2)})
		jsonAssert(t, body, map[string]any{"data.3.rank": 4})
	}

	// just user_id 2
	{
		status, body := testGet(t, app, "/v1/users/7eP5n/supporters/ML51L")
		assert.Equal(t, 200, status)
		jsonAssert(t, body, map[string]any{"data.sender.id": trashid.MustEncodeHashID(2)})
		jsonAssert(t, body, map[string]any{"data.rank": 4})
	}
}
