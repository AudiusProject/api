package api

import (
	"testing"
	"time"

	"api.audius.co/database"
	"github.com/stretchr/testify/assert"
)

func TestV1ChallengesInfo(t *testing.T) {
	app := emptyTestApp(t)

	now := time.Now().UTC()

	fixtures := database.FixtureMap{
		"challenges": {
			{
				"id":             "challenge-aggregate",
				"type":           "aggregate",
				"amount":         "100000000",
				"active":         true,
				"step_count":     10,
				"starting_block": 100,
				"weekly_pool":    100,
				"cooldown_days":  7,
			},
		},
		"users": {
			{"user_id": 1, "handle": "user1", "handle_lc": "user1", "wallet": "0xwallet1"},
		},
		"sol_reward_disbursements": {
			{
				"challenge_id":          "challenge-aggregate",
				"specifier":             "spec-a",
				"signature":             "sig-a",
				"instruction_index":     0,
				"slot":                  1,
				"amount":                200000000,
				"recipient_eth_address": "0xwallet1",
				"created_at":            now,
			},
		},
	}

	database.Seed(app.pool.Replicas[0], fixtures)

	status, body := testGet(t, app, "/v1/challenges/challenge-aggregate/info")
	assert.Equal(t, 200, status)
	jsonAssert(t, body, map[string]any{
		"data.challenge_id":          "challenge-aggregate",
		"data.type":                  "aggregate",
		"data.amount":                "100000000",
		"data.weekly_pool":           100,
		"data.weekly_pool_remaining": 98,
	})

	status, body = testGet(t, app, "/v1/challenges/challenge-aggregate/info?weekly_pool_min_amount=99")
	assert.Equal(t, 500, status)
	jsonAssert(t, body, map[string]any{
		"data.challenge_id":          "challenge-aggregate",
		"data.weekly_pool_remaining": 98,
	})
}

func TestV1ChallengesInfoInvalidWeeklyPoolMinAmount(t *testing.T) {
	app := emptyTestApp(t)

	status, body := testGet(t, app, "/v1/challenges/any/info?weekly_pool_min_amount=-1")
	assert.Equal(t, 400, status)
	jsonAssert(t, body, map[string]any{
		"error": "weekly_pool_min_amount is invalid",
	})
}
