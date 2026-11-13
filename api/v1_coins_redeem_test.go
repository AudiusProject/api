package api

import (
	"context"
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
				"remaining_uses": 1,
				"created_at":     time.Now(),
			},
			{
				"code":           "CODE456",
				"mint":           "TestMint123",
				"reward_address": "RewardAddress456",
				"amount":         250,
				"remaining_uses": 1,
				"created_at":     time.Now(),
			},
			{
				"code":           "USEDCODE",
				"mint":           "TestMint123",
				"reward_address": "RewardAddress789",
				"amount":         500,
				"remaining_uses": 0,
				"created_at":     time.Now(),
			},
		},
	}

	database.Seed(app.pool.Replicas[0], fixtures)

	for _, path := range []string{"/v1/coins/TestMint123/redeem", "/v1/coins/TestMint123/redeem/CODE123", "/v1/coins/TestMint123/redeem/USEDCODE"} {
		status, _ := testGet(t, app, path)
		assert.Equal(t, 410, status)
		status, _ = testPost(t, app, path, []byte(`{}`), nil)
		assert.Equal(t, 410, status)
	}
	var remaining int
	err := app.pool.QueryRow(context.Background(), "SELECT remaining_uses FROM reward_codes WHERE code = 'CODE123'").Scan(&remaining)
	assert.NoError(t, err)
	assert.Equal(t, 1, remaining)
}
