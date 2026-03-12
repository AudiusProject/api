package api

import (
	"testing"

	"api.audius.co/database"
	"github.com/stretchr/testify/assert"
)

func TestV1CoinMembersCount(t *testing.T) {
	app := emptyTestApp(t)

	fixtures := database.FixtureMap{
		"artist_coins": {
			{
				"mint":     "coin-mint-1",
				"ticker":   "COINONE",
				"user_id":  1,
				"decimals": 2,
			},
		},
		"sol_user_balances": {
			{
				"user_id": 1,
				"mint":    "coin-mint-1",
				"balance": 99,
				"created_at": parseTimeWithLayout(
					t,
					"2024-01-01 00:00:00",
					"2006-01-02 15:04:05",
				),
				"updated_at": parseTimeWithLayout(
					t,
					"2024-01-01 00:00:00",
					"2006-01-02 15:04:05",
				),
			},
			{
				"user_id": 2,
				"mint":    "coin-mint-1",
				"balance": 100,
				"created_at": parseTimeWithLayout(
					t,
					"2024-01-01 00:00:00",
					"2006-01-02 15:04:05",
				),
				"updated_at": parseTimeWithLayout(
					t,
					"2024-01-01 00:00:00",
					"2006-01-02 15:04:05",
				),
			},
			{
				"user_id": 3,
				"mint":    "coin-mint-1",
				"balance": 250,
				"created_at": parseTimeWithLayout(
					t,
					"2024-01-01 00:00:00",
					"2006-01-02 15:04:05",
				),
				"updated_at": parseTimeWithLayout(
					t,
					"2024-01-01 00:00:00",
					"2006-01-02 15:04:05",
				),
			},
		},
	}

	database.Seed(app.pool.Replicas[0], fixtures)

	status, body := testGet(t, app, "/v1/coins/coin-mint-1/members/count")
	assert.Equal(t, 200, status)
	jsonAssert(t, body, map[string]any{
		"data": 2,
	})

	status, body = testGet(t, app, "/v1/coins/coin-mint-1/members/count?min_balance=2.5")
	assert.Equal(t, 200, status)
	jsonAssert(t, body, map[string]any{
		"data": 1,
	})
}

func TestV1CoinMembersCountInvalidMinBalance(t *testing.T) {
	app := emptyTestApp(t)

	status, body := testGet(t, app, "/v1/coins/coin-mint-1/members/count?min_balance=-1")
	assert.Equal(t, 400, status)
	jsonAssert(t, body, map[string]any{
		"error": "min_balance is invalid",
	})
}
