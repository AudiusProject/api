package challenges

import (
	"context"
	"fmt"
	"testing"

	"api.audius.co/database"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// seedUserEvent inserts a current user_events row.
func seedUserEvent(t *testing.T, pool *pgxpool.Pool, userID int64, mobile bool, referrer *int64) {
	t.Helper()
	_, err := pool.Exec(context.Background(), `
		INSERT INTO user_events (user_id, is_mobile_user, referrer, is_current, blocknumber, blockhash)
		VALUES ($1, $2, $3, true, 101, 'block1')
	`, userID, mobile, referrer)
	require.NoError(t, err)
}

func TestMobileInstall_AwardsFromUserEvents(t *testing.T) {
	pool := withChallengesDB(t)
	database.Seed(pool, database.FixtureMap{}) // seeds block 101
	seedUserEvent(t, pool, 700, true, nil)
	seedUserEvent(t, pool, 701, false, nil) // not a mobile user → no award

	runProcessor(t, pool, &MobileInstallProcessor{})

	r, ok := queryUserChallenge(t, pool, "m", fmt.Sprintf("%x", 700))
	if assert.True(t, ok, "mobile user 700 should be awarded") {
		assert.True(t, r.IsComplete)
		assert.Equal(t, int32(1), r.Amount)
	}
	_, ok = queryUserChallenge(t, pool, "m", fmt.Sprintf("%x", 701))
	assert.False(t, ok, "non-mobile user 701 should have no row")
}

func TestMobileInstall_Idempotent(t *testing.T) {
	pool := withChallengesDB(t)
	database.Seed(pool, database.FixtureMap{})
	seedUserEvent(t, pool, 700, true, nil)

	runProcessor(t, pool, &MobileInstallProcessor{})
	runProcessor(t, pool, &MobileInstallProcessor{}) // re-run must not change state

	var count int
	require.NoError(t, pool.QueryRow(context.Background(),
		`SELECT COUNT(*) FROM user_challenges WHERE challenge_id = 'm' AND specifier = $1`,
		fmt.Sprintf("%x", 700)).Scan(&count))
	assert.Equal(t, 1, count, "exactly one row after two runs")
}

func TestReferral_VerifiedReferrer(t *testing.T) {
	pool := withChallengesDB(t)
	database.Seed(pool, database.FixtureMap{
		"users": {
			{"user_id": 500, "wallet": "0x500", "is_verified": true}, // referrer
		},
	})
	ref := int64(500)
	seedUserEvent(t, pool, 600, false, &ref) // user 600 referred by verified 500

	runProcessor(t, pool, NewVerifiedReferralProcessor()) // "rv"
	runProcessor(t, pool, NewReferralProcessor())         // "r"

	// rv awarded to the referrer (500), specifier referrer:referred.
	r, ok := queryUserChallenge(t, pool, "rv", fmt.Sprintf("%x:%x", 500, 600))
	if assert.True(t, ok, "verified referrer should earn rv") {
		assert.True(t, r.IsComplete)
	}
	// r (unverified tier) must NOT award for a verified referrer.
	_, ok = queryUserChallenge(t, pool, "r", fmt.Sprintf("%x:%x", 500, 600))
	assert.False(t, ok, "verified referrer must not earn the unverified r tier")
}

func TestReferral_UnverifiedReferrer(t *testing.T) {
	pool := withChallengesDB(t)
	database.Seed(pool, database.FixtureMap{
		"users": {
			{"user_id": 501, "wallet": "0x501", "is_verified": false}, // referrer
		},
	})
	ref := int64(501)
	seedUserEvent(t, pool, 601, false, &ref)

	runProcessor(t, pool, NewReferralProcessor())         // "r"
	runProcessor(t, pool, NewVerifiedReferralProcessor()) // "rv"

	r, ok := queryUserChallenge(t, pool, "r", fmt.Sprintf("%x:%x", 501, 601))
	if assert.True(t, ok, "unverified referrer should earn r") {
		assert.True(t, r.IsComplete)
	}
	_, ok = queryUserChallenge(t, pool, "rv", fmt.Sprintf("%x:%x", 501, 601))
	assert.False(t, ok, "unverified referrer must not earn the verified rv tier")
}

func TestReferred_AwardsReferredUser(t *testing.T) {
	pool := withChallengesDB(t)
	database.Seed(pool, database.FixtureMap{})
	ref := int64(500)
	seedUserEvent(t, pool, 600, false, &ref)

	runProcessor(t, pool, &ReferredProcessor{})

	// rd awarded to the referred user (600), specifier referred:referrer.
	r, ok := queryUserChallenge(t, pool, "rd", fmt.Sprintf("%x:%x", 600, 500))
	if assert.True(t, ok, "referred user should earn rd") {
		assert.True(t, r.IsComplete)
	}
}

// Pre-seeding the checkpoint past a user_events row's blocknumber must
// cause the dirty scan to skip it — this is what makes the prod backfill
// safe on first deploy (migration 0211 sets checkpoints to current max).
func TestMobileInstall_SkipsRowsBelowCheckpoint(t *testing.T) {
	pool := withChallengesDB(t)
	database.Seed(pool, database.FixtureMap{}) // seeds block 101
	seedUserEvent(t, pool, 700, true, nil)     // mobile user at block 101

	// Set the checkpoint past the seeded row's blocknumber.
	_, err := pool.Exec(context.Background(), `
		INSERT INTO indexing_checkpoints (tablename, last_checkpoint)
		VALUES ('challenges:m:last_blocknumber', 200)
		ON CONFLICT (tablename) DO UPDATE SET last_checkpoint = EXCLUDED.last_checkpoint
	`)
	require.NoError(t, err)

	runProcessor(t, pool, &MobileInstallProcessor{})

	_, ok := queryUserChallenge(t, pool, "m", fmt.Sprintf("%x", 700))
	assert.False(t, ok, "row below checkpoint must not be reprocessed")
}
