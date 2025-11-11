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
			{
				"user_id":         5,
				"handle":          "user5",
				"coin_flair_mint": "test_mint_address_124", // Prefers STEVE
			},
			{
				"user_id":         6,
				"handle":          "user6",
				"coin_flair_mint": "test_mint_address_123", // Prefers TESTCOIN but has zero balance
			},
			{
				"user_id":         7,
				"handle":          "user7",
				"coin_flair_mint": "", // Empty string - should show no badge
			},
			{
				"user_id":         8,
				"handle":          "user8",
				"coin_flair_mint": "test_mint_address_124", // Prefers STEVE over their own coin
			},
		},
		"artist_coins": {
			{
				"ticker":     "TESTCOIN",
				"decimals":   8,
				"user_id":    3,
				"mint":       "test_mint_address_123",
				"logo_uri":   "https://example.com/test-logo.png",
				"created_at": "2024-01-01 00:00:00",
			},
			{
				"ticker":     "STEVE",
				"decimals":   8,
				"user_id":    4,
				"mint":       "test_mint_address_124",
				"logo_uri":   "https://example.com/steve-logo.png",
				"created_at": "2024-01-01 00:00:00",
			},
			{
				"ticker":     "AUDIO",
				"decimals":   8,
				"user_id":    3,
				"mint":       "9LzCMqDgTKYz9Drzqnpgee3SGa89up3a247ypMj2xrqM",
				"logo_uri":   "https://example.com/audio-logo.png",
				"created_at": "2024-01-01 00:00:00",
			},
			{
				"ticker":     "USER8COIN",
				"decimals":   8,
				"user_id":    8,
				"mint":       "test_mint_address_125",
				"logo_uri":   "https://example.com/user8-logo.png",
				"created_at": "2024-01-01 00:00:00",
			},
		},
		"sol_user_balances": {
			// User 1: TESTCOIN has higher value (100 * $2 = $200) than STEVE (50 * $1 = $50)
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
			// User 3: owns TESTCOIN (takes priority regardless of value)
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
			// User 4 owns $STEVE but has zero balance in it, has other coins
			{
				"user_id": 4,
				"mint":    "test_mint_address_124",
				"balance": 0,
			},
			{
				"user_id": 4,
				"mint":    "test_mint_address_123",
				"balance": 300,
			},
			// User 5 prefers STEVE and has balance in it (should show STEVE even if they have higher value of other coins)
			{
				"user_id": 5,
				"mint":    "test_mint_address_124",
				"balance": 50,
			},
			{
				"user_id": 5,
				"mint":    "test_mint_address_123",
				"balance": 1000, // Much higher value but should be ignored due to preference
			},
			{
				"user_id": 5,
				"mint":    "9LzCMqDgTKYz9Drzqnpgee3SGa89up3a247ypMj2xrqM",
				"balance": 500,
			},
			// User 6 prefers TESTCOIN but has zero balance (should fall back to highest value coin)
			{
				"user_id": 6,
				"mint":    "test_mint_address_123",
				"balance": 0,
			},
			{
				"user_id": 6,
				"mint":    "test_mint_address_124",
				"balance": 200,
			},
			// User 7 has empty string preference (should show no badge)
			{
				"user_id": 7,
				"mint":    "test_mint_address_123",
				"balance": 100,
			},
			{
				"user_id": 7,
				"mint":    "test_mint_address_124",
				"balance": 200,
			},
			// User 8 prefers STEVE over their own coin (should show STEVE despite having their own coin)
			{
				"user_id": 8,
				"mint":    "test_mint_address_124", // STEVE - preferred coin
				"balance": 100,
			},
			{
				"user_id": 8,
				"mint":    "test_mint_address_125", // Their own coin
				"balance": 500,                     // Higher balance but should be ignored due to preference
			},
		},
		"artist_coin_stats": {
			// TESTCOIN has higher price per token, making user 1's 100 TESTCOIN more valuable than 50 STEVE
			{
				"mint":  "test_mint_address_123", // TESTCOIN
				"price": 2.0,                     // $2 per token (decimals=8)
			},
			{
				"mint":  "test_mint_address_124", // STEVE
				"price": 1.0,                     // $1 per token (decimals=8)
			},
			{
				"mint":  "test_mint_address_125", // USER8COIN
				"price": 0.5,                     // $0.5 per token (decimals=8)
			},
		},
	}

	database.Seed(app.writePool, fixtures)

	// Default badge should ignore AUDIO and return highest value (balance * price)
	// User 1: 100 TESTCOIN @ $2 = 200 value, 50 STEVE @ $1 = 50 value
	{
		status, body := testGet(t, app, "/v1/full/users/"+trashid.MustEncodeHashID(1))
		assert.Equal(t, 200, status)

		jsonAssert(t, body, map[string]any{
			"data.0.coin_flair_mint":            nil,
			"data.0.artist_coin_badge.mint":     "test_mint_address_123",
			"data.0.artist_coin_badge.ticker":   "TESTCOIN",
			"data.0.artist_coin_badge.logo_uri": "https://example.com/test-logo.png",
		})
	}

	// Ignore zero balance coins for badges
	{
		status, body := testGet(t, app, "/v1/full/users/"+trashid.MustEncodeHashID(2))
		assert.Equal(t, 200, status)

		jsonAssert(t, body, map[string]any{
			"data.0.coin_flair_mint":   nil,
			"data.0.artist_coin_badge": nil,
		})
	}

	// Return own artist coin first even if not highest value
	{
		status, body := testGet(t, app, "/v1/full/users/"+trashid.MustEncodeHashID(3))
		assert.Equal(t, 200, status)

		jsonAssert(t, body, map[string]any{
			"data.0.coin_flair_mint":            nil,
			"data.0.artist_coin_badge.mint":     "test_mint_address_123",
			"data.0.artist_coin_badge.ticker":   "TESTCOIN",
			"data.0.artist_coin_badge.logo_uri": "https://example.com/test-logo.png",
		})
	}

	// Always show artist's created coin even with zero balance
	{
		status, body := testGet(t, app, "/v1/full/users/"+trashid.MustEncodeHashID(4))
		assert.Equal(t, 200, status)

		jsonAssert(t, body, map[string]any{
			"data.0.coin_flair_mint":            nil,
			"data.0.artist_coin_badge.mint":     "test_mint_address_124",
			"data.0.artist_coin_badge.ticker":   "STEVE",
			"data.0.artist_coin_badge.logo_uri": "https://example.com/steve-logo.png",
		})
	}

	// Preferred flair with non-zero balance takes priority over higher value coins
	{
		status, body := testGet(t, app, "/v1/full/users/"+trashid.MustEncodeHashID(5))
		assert.Equal(t, 200, status)

		jsonAssert(t, body, map[string]any{
			"data.0.coin_flair_mint":            "test_mint_address_124",
			"data.0.artist_coin_badge.mint":     "test_mint_address_124",
			"data.0.artist_coin_badge.ticker":   "STEVE",
			"data.0.artist_coin_badge.logo_uri": "https://example.com/steve-logo.png",
		})
	}

	// Preferred flair with zero balance falls back to 'auto' logic (artist's own coin/highest value)
	{
		status, body := testGet(t, app, "/v1/full/users/"+trashid.MustEncodeHashID(6))
		assert.Equal(t, 200, status)

		jsonAssert(t, body, map[string]any{
			"data.0.coin_flair_mint":            "test_mint_address_123",
			"data.0.artist_coin_badge.mint":     "test_mint_address_124",
			"data.0.artist_coin_badge.ticker":   "STEVE",
			"data.0.artist_coin_badge.logo_uri": "https://example.com/steve-logo.png",
		})
	}

	// Empty string preferred flair should return no badge even if user has balances
	{
		status, body := testGet(t, app, "/v1/full/users/"+trashid.MustEncodeHashID(7))
		assert.Equal(t, 200, status)

		jsonAssert(t, body, map[string]any{
			"data.0.coin_flair_mint":   "",
			"data.0.artist_coin_badge": nil,
		})
	}

	// Preferred flair takes priority over user's own artist coin
	{
		status, body := testGet(t, app, "/v1/full/users/"+trashid.MustEncodeHashID(8))
		assert.Equal(t, 200, status)

		jsonAssert(t, body, map[string]any{
			"data.0.artist_coin_badge.mint":     "test_mint_address_124",
			"data.0.artist_coin_badge.ticker":   "STEVE",
			"data.0.artist_coin_badge.logo_uri": "https://example.com/steve-logo.png",
		})
	}
}
