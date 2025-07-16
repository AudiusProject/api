package api

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetUndisbursedChallenges(t *testing.T) {
	app := testAppWithFixtures(t)

	// Test basic functionality with no parameters
	status, body := testGet(t, app, "/v1/challenges/undisbursed")
	assert.Equal(t, 200, status)
	jsonAssert(t, body, map[string]any{
		"data.0.challenge_id":  "e",
		"data.0.user_id":       "eP7kD",
		"data.0.specifier":     "def",
		"data.0.amount":        "3",
		"data.0.handle":        "challenges_finished_streak",
		"data.0.wallet":        "0x001",
		"data.0.cooldown_days": 0,
	})

	// Test filtering by user_id
	status, body = testGetWithWallet(t, app,
		"/v1/challenges/undisbursed?user_id=L50xn",
		"0x8A2c4dcb2Eb9c2C5bc6E28310E4B07011D230C0A",
	)
	assert.Equal(t, 200, status)
	jsonAssert(t, body, map[string]any{
		"data.0.user_id": "L50xn",
	})

	// Test filtering by challenge_id
	status, body = testGet(t, app, "/v1/challenges/undisbursed?challenge_id=f")
	assert.Equal(t, 200, status)
	jsonAssert(t, body, map[string]any{
		"data.0.challenge_id": "f",
	})

	// Test pagination with limit and offset
	status, body = testGet(t, app, "/v1/challenges/undisbursed?limit=1&offset=1")
	assert.Equal(t, 200, status)
	// Verify we get the second item
	jsonAssert(t, body, map[string]any{
		"data.0.user_id": "L50xn",
	})

	// Test invalid parameters
	status, _ = testGet(t, app, "/v1/challenges/undisbursed?limit=invalid")
	assert.Equal(t, 400, status)

	status, _ = testGet(t, app, "/v1/challenges/undisbursed?offset=invalid")
	assert.Equal(t, 400, status)

	status, _ = testGet(t, app, "/v1/challenges/undisbursed?completed_blocknumber=invalid")
	assert.Equal(t, 400, status)
}
