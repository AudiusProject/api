package api

import (
	"testing"
	"time"

	"bridgerton.audius.co/database"
	"bridgerton.audius.co/trashid"
	"github.com/stretchr/testify/assert"
)

func TestGetTips(t *testing.T) {
	app := emptyTestApp(t)

	fixtures := database.FixtureMap{
		"users": {
			{
				"user_id": 1,
				"handle":  "user1",
			},
			{
				"user_id": 2,
				"handle":  "user2",
			},
			{
				"user_id": 3,
				"handle":  "DontShowMyTips",
			},
			{
				"user_id": 4,
				"handle":  "user4",
			},
		},
		"follows": {
			{
				"follower_user_id": 1,
				"followee_user_id": 2,
			},
			{
				"follower_user_id": 1,
				"followee_user_id": 3,
			},
		},
		"user_tips": {
			{
				"slot":             0,
				"signature":        "abcdefg",
				"sender_user_id":   1,
				"receiver_user_id": 2,
				"amount":           100000000,
				"created_at":       time.Now().Add(-2 * time.Hour),
				"updated_at":       time.Now().Add(-2 * time.Hour),
			},
			{
				"slot":             1,
				"signature":        "hijklmn",
				"sender_user_id":   1,
				"receiver_user_id": 3,
				"amount":           200000000,
				"created_at":       time.Now().Add(-1 * time.Hour),
				"updated_at":       time.Now().Add(-1 * time.Hour),
			},
			{
				"slot":             2,
				"signature":        "opqrstu",
				"sender_user_id":   2,
				"receiver_user_id": 3,
				"amount":           150000000,
				"created_at":       time.Now(),
				"updated_at":       time.Now(),
			},
		},
		"aggregate_user_tips": {
			{
				"sender_user_id":   1,
				"receiver_user_id": 2,
				"amount":           100000000,
			},
			{
				"sender_user_id":   1,
				"receiver_user_id": 3,
				"amount":           200000000,
			},
			{
				"sender_user_id":   2,
				"receiver_user_id": 3,
				"amount":           150000000,
			},
		},
		"aggregate_user": {
			{
				"user_id":        1,
				"follower_count": 100,
			},
			{
				"user_id":        2,
				"follower_count": 50,
			},
			{
				"user_id":        3,
				"follower_count": 200,
			},
			{
				"user_id":        4,
				"follower_count": 5,
			},
		},
	}

	database.Seed(app.pool, fixtures)

	// Basic test - get all tips without authentication
	{
		status, body := testGet(t, app, "/v1/full/tips")
		assert.Equal(t, 200, status)

		jsonAssert(t, body, map[string]any{
			"data.0.sender.id":    trashid.MustEncodeHashID(2),
			"data.0.receiver.id":  trashid.MustEncodeHashID(3),
			"data.0.amount":       "1500000000000000000",
			"data.0.slot":         2,
			"data.0.tx_signature": "opqrstu",
			"data.1.sender.id":    trashid.MustEncodeHashID(1),
			"data.1.receiver.id":  trashid.MustEncodeHashID(3),
			"data.1.amount":       "2000000000000000000",
			"data.1.slot":         1,
			"data.1.tx_signature": "hijklmn",
			"data.2.sender.id":    trashid.MustEncodeHashID(1),
			"data.2.receiver.id":  trashid.MustEncodeHashID(2),
			"data.2.amount":       "1000000000000000000",
			"data.2.slot":         0,
			"data.2.tx_signature": "abcdefg",
		})
	}

	// Test with limit and offset
	{
		status, body := testGet(t, app, "/v1/full/tips?limit=1&offset=1")
		assert.Equal(t, 200, status)

		jsonAssert(t, body, map[string]any{
			"data.0.sender.id":    trashid.MustEncodeHashID(1),
			"data.0.receiver.id":  trashid.MustEncodeHashID(3),
			"data.0.amount":       "2000000000000000000",
			"data.0.slot":         1,
			"data.0.tx_signature": "hijklmn",
			"data.1":              nil,
		})
	}

	// Test filtering by min_slot
	{
		status, body := testGet(t, app, "/v1/full/tips?min_slot=1")
		assert.Equal(t, 200, status)

		jsonAssert(t, body, map[string]any{
			"data.0.slot": 2,
			"data.1.slot": 1,
			"data.2":      nil, // Should not include slot 0
		})
	}

	// Test filtering by max_slot
	{
		status, body := testGet(t, app, "/v1/full/tips?max_slot=1")
		assert.Equal(t, 200, status)

		jsonAssert(t, body, map[string]any{
			"data.0.slot": 1,
			"data.1.slot": 0,
			"data.2":      nil, // Should not include slot 2
		})
	}

	// Test filtering by signatures
	{
		status, body := testGet(t, app, "/v1/full/tips?tx_signature=hijklmn&tx_signature=opqrstu")
		assert.Equal(t, 200, status)

		jsonAssert(t, body, map[string]any{
			"data.#":              2,
			"data.0.tx_signature": "opqrstu",
			"data.1.tx_signature": "hijklmn",
		})
	}

	// Test filtering by receiver_min_followers
	{
		status, body := testGet(t, app, "/v1/full/tips?receiver_min_followers=100")
		assert.Equal(t, 200, status)

		// Should only include tips to user 3 (follower_count: 200)
		jsonAssert(t, body, map[string]any{
			"data.0.receiver.id": trashid.MustEncodeHashID(3),
			"data.1.receiver.id": trashid.MustEncodeHashID(3),
			"data.2":             nil, // Should not include tips to user 2 (follower_count: 50)
		})
	}
}
