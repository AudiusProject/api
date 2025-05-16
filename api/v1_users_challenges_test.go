package api

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUserChallenges(t *testing.T) {
	status, body := testGet(t, "/v1/users/eYVJn/challenges")
	assert.Equal(t, 200, status)
	jsonAssert(t, body, map[string]any{
		"data.0.challenge_id":       "e",
		"data.0.user_id":            "eYVJn",
		"data.0.amount":             "1",
		"data.0.current_step_count": "2",
		"data.0.is_complete":        false,
	})

	// Completed endless challenge
	status, body = testGet(t, "/v1/users/eP7kD/challenges")
	assert.Equal(t, 200, status)
	jsonAssert(t, body, map[string]any{
		"data.0.challenge_id":       "e",
		"data.0.user_id":            "eP7kD",
		"data.0.amount":             "1",
		"data.0.current_step_count": "3",
		"data.0.is_complete":        true,
	})

	// Continued endless challenge
	status, body = testGet(t, "/v1/users/L50xn/challenges")
	assert.Equal(t, 200, status)
	jsonAssert(t, body, map[string]any{
		"data.0.challenge_id":       "e",
		"data.0.user_id":            "L50xn",
		"data.0.amount":             "1",
		"data.0.current_step_count": "5",
		"data.0.is_complete":        true,
	})

	// Reset endless challenge
	status, body = testGet(t, "/v1/users/eblKL/challenges")
	assert.Equal(t, 200, status)
	jsonAssert(t, body, map[string]any{
		"data.0.challenge_id":       "e",
		"data.0.user_id":            "eblKL",
		"data.0.amount":             "1",
		"data.0.current_step_count": "0",
		"data.0.is_complete":        false,
	})
}
