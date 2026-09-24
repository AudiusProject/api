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

func TestWeeklyPoolWindowStartAt(t *testing.T) {
	utc := func(y int, m time.Month, d, h int) time.Time {
		return time.Date(y, m, d, h, 0, 0, 0, time.UTC)
	}
	// 2026-09-07 is a Monday.
	thisMonday := utc(2026, time.September, 7, 16)
	lastMonday := utc(2026, time.August, 31, 16)

	cases := []struct {
		name string
		now  time.Time
		want time.Time
	}{
		{"Monday before 16:00 is still last week", utc(2026, time.September, 7, 15), lastMonday},
		{"Monday at 16:00 opens the week", thisMonday, thisMonday},
		{"Wednesday", utc(2026, time.September, 9, 12), thisMonday},
		{"Saturday", utc(2026, time.September, 12, 23), thisMonday},
		{"Sunday", utc(2026, time.September, 13, 21), thisMonday},
		{"Sunday just before the next Monday", utc(2026, time.September, 13, 23), thisMonday},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, weeklyPoolWindowStartAt(tc.now))
		})
	}
}
