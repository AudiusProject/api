package challenges

import (
	"context"
	"fmt"
	"testing"

	"api.audius.co/database"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCosign_VerifiedParentReposting — happy path: verified parent owner
// reposts a remix of their own track → remixer earns 'cs'.
func TestCosign_VerifiedParentReposting(t *testing.T) {
	pool := withChallengesDB(t)
	ctx := context.Background()

	// 'cs' is active=false by default. Enable for the test.
	_, err := pool.Exec(ctx, "UPDATE challenges SET active = true WHERE id = 'cs'")
	require.NoError(t, err)

	database.Seed(pool, database.FixtureMap{
		"blocks": {{"blockhash": "blk_cs", "number": 1}},
		"users": {
			{"user_id": 1000, "wallet": "0x1000", "is_verified": true},  // parent owner (cosigner)
			{"user_id": 1001, "wallet": "0x1001", "is_verified": false}, // remixer
		},
		"tracks": {
			{"track_id": 100000, "owner_id": 1000, "title": "Parent", "blocknumber": 1},
			{"track_id": 100001, "owner_id": 1001, "title": "Remix", "blocknumber": 1},
		},
	})
	// Seed remixes + parent owner reposts the remix.
	_, err = pool.Exec(ctx, `
		INSERT INTO remixes (parent_track_id, child_track_id) VALUES (100000, 100001)
	`)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `
		INSERT INTO reposts (user_id, repost_item_id, repost_type, is_current, is_delete, created_at, txhash, blocknumber)
		VALUES (1000, 100001, 'track', true, false, now(), 'tx-r1', 1)
	`)
	require.NoError(t, err)

	runProcessor(t, pool, &CosignProcessor{})

	r, ok := queryUserChallenge(t, pool, "cs", fmt.Sprintf("%x:%x", 1000, 100001))
	if assert.True(t, ok) {
		assert.True(t, r.IsComplete)
		assert.Equal(t, int64(1001), r.UserID, "row filed under the remixer")
		assert.Equal(t, int32(1000), r.Amount)
	}
}

// TestCosign_MonthCap — 5 cosigns per parent-owner per rolling 30 days.
// Seed 5 already-completed user_challenges to simulate the cap and verify
// a 6th candidate is skipped.
func TestCosign_MonthCap(t *testing.T) {
	pool := withChallengesDB(t)
	ctx := context.Background()
	_, err := pool.Exec(ctx, "UPDATE challenges SET active = true WHERE id = 'cs'")
	require.NoError(t, err)

	database.Seed(pool, database.FixtureMap{
		"blocks": {{"blockhash": "blk_cs2", "number": 1}},
		"users": {
			{"user_id": 1100, "wallet": "0x1100", "is_verified": true},
			{"user_id": 1101, "wallet": "0x1101"},
		},
		"tracks": {
			{"track_id": 110000, "owner_id": 1100, "title": "Parent", "blocknumber": 1},
			{"track_id": 110001, "owner_id": 1101, "title": "Remix6", "blocknumber": 1},
		},
	})

	// Pre-populate 5 cosigns for parent 1100 within the last 30 days.
	// completed_at must be non-null so the on_user_challenge trigger can
	// write its notification with a non-null timestamp.
	for i := 0; i < 5; i++ {
		spec := fmt.Sprintf("%x:%x", 1100, 50000+i) // arbitrary remix ids
		_, err := pool.Exec(ctx, `
			INSERT INTO user_challenges (challenge_id, user_id, specifier, is_complete, amount, created_at, completed_at)
			VALUES ('cs', $1, $2, true, 1000, now(), now())
		`, 5000+i, spec)
		require.NoError(t, err)
	}
	// Add the would-be 6th candidate.
	_, err = pool.Exec(ctx, `
		INSERT INTO remixes (parent_track_id, child_track_id) VALUES (110000, 110001)
	`)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `
		INSERT INTO reposts (user_id, repost_item_id, repost_type, is_current, is_delete, created_at, txhash, blocknumber)
		VALUES (1100, 110001, 'track', true, false, now(), 'tx-r6', 1)
	`)
	require.NoError(t, err)

	runProcessor(t, pool, &CosignProcessor{})

	_, ok := queryUserChallenge(t, pool, "cs", fmt.Sprintf("%x:%x", 1100, 110001))
	assert.False(t, ok, "6th cosign within 30 days should be blocked by cap")
}
