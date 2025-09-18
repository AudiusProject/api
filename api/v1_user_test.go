package api

import (
	"testing"

	"api.audius.co/api/dbv1"
	"api.audius.co/database"
	"api.audius.co/trashid"
	"github.com/stretchr/testify/assert"
)

func TestGetUser(t *testing.T) {
	app := testAppWithFixtures(t)
	var userResponse struct {
		Data []dbv1.FullUser
	}

	status, body := testGet(t, app, "/v1/full/users/7eP5n", &userResponse)
	assert.Equal(t, 200, status)

	// body is response json
	jsonAssert(t, body, map[string]any{
		"data.0.handle":  "rayjacobson",
		"data.0.user_id": 1,
		"data.0.id":      "7eP5n",
	})

	// but we also unmarshaled into userResponse
	// for structured testing
	assert.Equal(t, userResponse.Data[0].ID, trashid.HashId(1))

	// test that we can get a user by handle
	status, body = testGet(t, app, "/v1/full/users/handle/rayjacobson")
	assert.Equal(t, 200, status)

	jsonAssert(t, body, map[string]any{
		"data.0.handle":  "rayjacobson",
		"data.0.user_id": 1,
		"data.0.id":      "7eP5n",
	})
}

func TestGetUserCoinBadges(t *testing.T) {
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
				"handle":  "user3",
			},
			{
				"user_id": 4,
				"handle":  "stereosteve",
			},
		},
		"artist_coins": {
			{
				"ticker":     "$TESTCOIN",
				"decimals":   8,
				"user_id":    3,
				"mint":       "test_mint_address_123",
				"logo_uri":   "https://example.com/test-logo.png",
				"created_at": "2024-01-01 00:00:00",
			},
			{
				"ticker":     "$STEVE",
				"decimals":   8,
				"user_id":    4,
				"mint":       "test_mint_address_124",
				"logo_uri":   "https://example.com/steve-logo.png",
				"created_at": "2024-01-01 00:00:00",
			},
			{
				"ticker":     "$AUDIO",
				"decimals":   8,
				"user_id":    3,
				"mint":       "9LzCMqDgTKYz9Drzqnpgee3SGa89up3a247ypMj2xrqM",
				"logo_uri":   "https://example.com/audio-logo.png",
				"created_at": "2024-01-01 00:00:00",
			},
		},
		"sol_user_balances": {
			// User 1 has more AUDIO than TESTCOIN, but more TESTCOIN than $TEVE
			{
				"user_id": 1,
				"mint":    "test_mint_address_123",
				"balance": 100,
			},
			{
				"user_id": 1,
				"mint":    "test_mint_address_124",
				"balance": 50,
			},
			{
				"user_id": 1,
				"mint":    "9LzCMqDgTKYz9Drzqnpgee3SGa89up3a247ypMj2xrqM",
				"balance": 200,
			},
			// User 2 used to have TESTCOIN but now only has AUDIO
			{
				"user_id": 2,
				"mint":    "test_mint_address_124",
				"balance": 0,
			},
			{
				"user_id": 2,
				"mint":    "9LzCMqDgTKYz9Drzqnpgee3SGa89up3a247ypMj2xrqM",
				"balance": 100,
			},
			// User 3 has more $TEVE then their own coin
			{
				"user_id": 3,
				"mint":    "test_mint_address_123",
				"balance": 100,
			},
			{
				"user_id": 3,
				"mint":    "test_mint_address_124",
				"balance": 500,
			},
		},
	}

	database.Seed(app.writePool, fixtures)

	// Default badge should ignore AUDIO and return highest balance
	{
		status, body := testGet(t, app, "/v1/full/users/"+trashid.MustEncodeHashID(1))
		assert.Equal(t, 200, status)

		jsonAssert(t, body, map[string]any{
			"data.0.artist_coin_badge.mint":     "test_mint_address_123",
			"data.0.artist_coin_badge.ticker":   "$TESTCOIN",
			"data.0.artist_coin_badge.logo_uri": "https://example.com/test-logo.png",
		})
	}

	// Ignore zero balance coins for badges
	{
		status, body := testGet(t, app, "/v1/full/users/"+trashid.MustEncodeHashID(2))
		assert.Equal(t, 200, status)

		jsonAssert(t, body, map[string]any{
			"data.0.artist_coin_badge": nil,
		})
	}

	// Return own artist coin first even if not highest balance
	{
		status, body := testGet(t, app, "/v1/full/users/"+trashid.MustEncodeHashID(3))
		assert.Equal(t, 200, status)

		jsonAssert(t, body, map[string]any{
			"data.0.artist_coin_badge.mint":     "test_mint_address_123",
			"data.0.artist_coin_badge.ticker":   "$TESTCOIN",
			"data.0.artist_coin_badge.logo_uri": "https://example.com/test-logo.png",
		})
	}
}
