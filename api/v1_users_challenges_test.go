package api

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUserChallenges(t *testing.T) {
	app := testAppWithFixtures(t)
	status, body := testGet(t, app, "/v1/users/eYVJn/challenges")
	assert.Equal(t, 200, status)
	jsonAssert(t, body, map[string]any{
		"data.0.challenge_id":       "e",
		"data.0.user_id":            "eYVJn",
		"data.0.amount":             "1",
		"data.0.current_step_count": "2",
		"data.0.is_complete":        false,
	})

	// Completed endless challenge
	status, body = testGet(t, app, "/v1/users/eP7kD/challenges")
	assert.Equal(t, 200, status)
	jsonAssert(t, body, map[string]any{
		"data.0.challenge_id":       "e",
		"data.0.user_id":            "eP7kD",
		"data.0.amount":             "1",
		"data.0.current_step_count": "3",
		"data.0.is_complete":        true,
	})

	// Continued endless challenge
	status, body = testGet(t, app, "/v1/users/L50xn/challenges")
	assert.Equal(t, 200, status)
	jsonAssert(t, body, map[string]any{
		"data.0.challenge_id":       "e",
		"data.0.user_id":            "L50xn",
		"data.0.amount":             "1",
		"data.0.current_step_count": "5",
		"data.0.is_complete":        true,
	})

	// Reset endless challenge
	status, body = testGet(t, app, "/v1/users/eblKL/challenges")
	assert.Equal(t, 200, status)
	jsonAssert(t, body, map[string]any{
		"data.0.challenge_id":       "e",
		"data.0.user_id":            "eblKL",
		"data.0.amount":             "1",
		"data.0.current_step_count": "0",
		"data.0.is_complete":        false,
	})
}

// A reward that has been disbursed for a (challenge_id, specifier) must show as
// disbursed to the user who completed that specifier, even when the on-chain
// recipient wallet resolves to a different user. Disbursements are deduped by
// (challenge_id, specifier) on-chain, so a paid specifier can never be claimed
// again — surfacing it as still-claimable produces a stuck reward that errors
// on claim.
func TestUserChallengesDisbursedToDifferentUser(t *testing.T) {
	app := testAppWithFixtures(t)
	ctx := context.Background()

	// User 402 (L50xn) completed boolean challenge "f" with specifier "fff".
	// Record an on-chain disbursement for that specifier whose recipient wallet
	// belongs to a different user (user 1, rayjacobson).
	_, err := app.pool.Exec(ctx, `
		INSERT INTO sol_reward_disbursements
			(signature, instruction_index, amount, slot, user_bank, challenge_id, specifier, recipient_eth_address)
		VALUES
			('sig-fff', 0, 100000000, 1, 'user-bank-x', 'f', 'fff', '0x7d273271690538cf855e5b3002a0dd8c154bb060')
	`)
	assert.NoError(t, err)

	status, body := testGet(t, app, "/v1/users/L50xn/challenges")
	assert.Equal(t, 200, status)
	// Challenges are ordered by challenge_id, so "f" follows the rolled-up "e".
	jsonAssert(t, body, map[string]any{
		"data.1.challenge_id":     "f",
		"data.1.is_complete":      true,
		"data.1.is_disbursed":     true,
		"data.1.disbursed_amount": float64(1),
	})
}
