package api

import (
	"testing"
	"time"

	"api.audius.co/database"
	"github.com/stretchr/testify/assert"
)

func TestV1CoinsRedeem(t *testing.T) {
	app := emptyTestApp(t)

	fixtures := database.FixtureMap{
		"artist_coins": {
			{
				"ticker":     "TEST",
				"decimals":   8,
				"user_id":    1,
				"mint":       "TestMint123",
				"name":       "Test Coin",
				"created_at": time.Now().Add(-time.Second),
			},
		},
		"reward_codes": {
			{
				"code":           "CODE123",
				"mint":           "TestMint123",
				"reward_address": "RewardAddress123",
				"amount":         100,
				"is_used":        false,
				"created_at":     time.Now(),
			},
			{
				"code":           "CODE456",
				"mint":           "TestMint123",
				"reward_address": "RewardAddress456",
				"amount":         250,
				"is_used":        false,
				"created_at":     time.Now(),
			},
			{
				"code":           "USEDCODE",
				"mint":           "TestMint123",
				"reward_address": "RewardAddress789",
				"amount":         500,
				"is_used":        true,
				"created_at":     time.Now(),
			},
		},
	}

	database.Seed(app.pool.Replicas[0], fixtures)

	// Test /coins/:mint/redeem endpoint - should always return amount: 1
	t.Run("GET /coins/:mint/redeem returns amount 1", func(t *testing.T) {
		status, body := testGet(t, app, "/v1/coins/TestMint123/redeem")
		assert.Equal(t, 200, status)

		jsonAssert(t, body, map[string]any{
			"amount": 1,
		})
	})

	// Test /coins/:mint/redeem/:code endpoint with valid code
	t.Run("GET /coins/:mint/redeem/:code with valid code", func(t *testing.T) {
		status, body := testGet(t, app, "/v1/coins/TestMint123/redeem/CODE123")
		assert.Equal(t, 200, status)

		jsonAssert(t, body, map[string]any{
			"code":   "CODE123",
			"amount": 100,
		})
	})

	// Test with another valid code
	t.Run("GET /coins/:mint/redeem/:code with another valid code", func(t *testing.T) {
		status, body := testGet(t, app, "/v1/coins/TestMint123/redeem/CODE456")
		assert.Equal(t, 200, status)

		jsonAssert(t, body, map[string]any{
			"code":   "CODE456",
			"amount": 250,
		})
	})

	// Test with used code
	t.Run("GET /coins/:mint/redeem/:code with used code returns 400 with error", func(t *testing.T) {
		status, body := testGet(t, app, "/v1/coins/TestMint123/redeem/USEDCODE")
		assert.Equal(t, 400, status)

		jsonAssert(t, body, map[string]any{
			"error": "used",
		})
	})

	// Test with non-existent code
	t.Run("GET /coins/:mint/redeem/:code with non-existent code returns 400", func(t *testing.T) {
		status, body := testGet(t, app, "/v1/coins/TestMint123/redeem/NONEXISTENT")
		assert.Equal(t, 400, status)
		jsonAssert(t, body, map[string]any{
			"error": "invalid",
		})
	})

	// Test with wrong mint for existing code
	t.Run("GET /coins/:mint/redeem/:code with wrong mint returns 400", func(t *testing.T) {
		status, body := testGet(t, app, "/v1/coins/WrongMint/redeem/CODE123")
		assert.Equal(t, 400, status)
		jsonAssert(t, body, map[string]any{
			"error": "invalid",
		})
	})

	// Test redeem endpoint with different mint
	t.Run("GET /coins/:mint/redeem works for any mint", func(t *testing.T) {
		status, body := testGet(t, app, "/v1/coins/AnyMintAddress/redeem")
		assert.Equal(t, 200, status)

		jsonAssert(t, body, map[string]any{
			"amount": 1,
		})
	})
}
