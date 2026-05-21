package challenges

import (
	"context"
	"fmt"
	"testing"

	"api.audius.co/database"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMobileInstall_OneRowPerUser(t *testing.T) {
	pool := withChallengesDB(t)
	ctx := context.Background()
	_, err := pool.Exec(ctx, `
		INSERT INTO challenge_signals (type, user_id, extra)
		VALUES ('mobile_install', 2000, '{}'::jsonb)
	`)
	require.NoError(t, err)

	runProcessor(t, pool, &MobileInstallProcessor{})

	r, ok := queryUserChallenge(t, pool, "m", fmt.Sprintf("%x", 2000))
	if assert.True(t, ok) {
		assert.True(t, r.IsComplete)
		assert.Equal(t, int32(1), r.Amount)
	}

	// Second run of the same processor without new signals is a no-op.
	runProcessor(t, pool, &MobileInstallProcessor{})
	var count int
	require.NoError(t, pool.QueryRow(ctx,
		"SELECT COUNT(*) FROM user_challenges WHERE challenge_id = 'm'").Scan(&count))
	assert.Equal(t, 1, count)
}

func TestOneShot_ExtraAmountOverrides(t *testing.T) {
	pool := withChallengesDB(t)
	ctx := context.Background()
	_, err := pool.Exec(ctx, `
		INSERT INTO challenge_signals (type, user_id, extra)
		VALUES ('one_shot', 2100, '{"amount": 500, "nonce": "drop-1"}'::jsonb)
	`)
	require.NoError(t, err)

	runProcessor(t, pool, &OneShotProcessor{})

	r, ok := queryUserChallenge(t, pool, "o", fmt.Sprintf("%x:drop-1", 2100))
	if assert.True(t, ok) {
		assert.Equal(t, int32(500), r.Amount, "extra.amount should override catalog amount")
		assert.True(t, r.IsComplete)
	}
}

func TestReferral_NonVerifiedReferrer(t *testing.T) {
	pool := withChallengesDB(t)
	ctx := context.Background()
	database.Seed(pool, database.FixtureMap{
		"blocks": {{"blockhash": "blk_ref", "number": 1}},
		"users": {
			{"user_id": 2200, "wallet": "0x2200", "is_verified": false}, // referrer
			{"user_id": 2201, "wallet": "0x2201"},                       // referred
		},
	})
	_, err := pool.Exec(ctx, `
		INSERT INTO challenge_signals (type, user_id, extra)
		VALUES ('referral', 2201, '{"referrer_user_id": 2200}'::jsonb)
	`)
	require.NoError(t, err)

	runProcessor(t, pool, NewReferralProcessor())          // r
	runProcessor(t, pool, NewVerifiedReferralProcessor())  // rv (should not fire)
	runProcessor(t, pool, &ReferredProcessor{})            // rd

	// r should mint for the referrer (2200).
	rRow, ok := queryUserChallenge(t, pool, "r", fmt.Sprintf("%x:%x", 2200, 2201))
	if assert.True(t, ok, "r row should exist") {
		assert.Equal(t, int64(2200), rRow.UserID, "row filed under referrer")
	}

	// rv should NOT mint (referrer not verified).
	_, ok = queryUserChallenge(t, pool, "rv", fmt.Sprintf("%x:%x", 2200, 2201))
	assert.False(t, ok, "rv gated on verified referrer")

	// rd should mint for the referred user.
	rdRow, ok := queryUserChallenge(t, pool, "rd", fmt.Sprintf("%x:%x", 2201, 2200))
	if assert.True(t, ok, "rd row should exist") {
		assert.Equal(t, int64(2201), rdRow.UserID, "row filed under referred user")
	}
}

func TestReferral_VerifiedReferrer(t *testing.T) {
	pool := withChallengesDB(t)
	ctx := context.Background()
	database.Seed(pool, database.FixtureMap{
		"blocks": {{"blockhash": "blk_refv", "number": 1}},
		"users": {
			{"user_id": 2210, "wallet": "0x2210", "is_verified": true}, // verified referrer
			{"user_id": 2211, "wallet": "0x2211"},
		},
	})
	_, err := pool.Exec(ctx, `
		INSERT INTO challenge_signals (type, user_id, extra)
		VALUES ('referral', 2211, '{"referrer_user_id": 2210}'::jsonb)
	`)
	require.NoError(t, err)

	runProcessor(t, pool, NewReferralProcessor())
	runProcessor(t, pool, NewVerifiedReferralProcessor())

	// r should NOT mint (referrer is verified).
	_, ok := queryUserChallenge(t, pool, "r", fmt.Sprintf("%x:%x", 2210, 2211))
	assert.False(t, ok)

	// rv SHOULD mint.
	rvRow, ok := queryUserChallenge(t, pool, "rv", fmt.Sprintf("%x:%x", 2210, 2211))
	if assert.True(t, ok) {
		assert.Equal(t, int64(2210), rvRow.UserID)
	}
}

func TestSignals_CheckpointAdvances(t *testing.T) {
	pool := withChallengesDB(t)
	ctx := context.Background()

	_, err := pool.Exec(ctx, `INSERT INTO challenge_signals (type, user_id, extra)
		VALUES ('mobile_install', 2300, '{}'::jsonb), ('mobile_install', 2301, '{}'::jsonb)`)
	require.NoError(t, err)

	runProcessor(t, pool, &MobileInstallProcessor{})

	// Add another signal after the first run.
	_, err = pool.Exec(ctx, `INSERT INTO challenge_signals (type, user_id, extra)
		VALUES ('mobile_install', 2302, '{}'::jsonb)`)
	require.NoError(t, err)

	runProcessor(t, pool, &MobileInstallProcessor{})

	for _, uid := range []int{2300, 2301, 2302} {
		_, ok := queryUserChallenge(t, pool, "m", fmt.Sprintf("%x", uid))
		assert.True(t, ok, "user %d should have mobile_install row", uid)
	}
}
