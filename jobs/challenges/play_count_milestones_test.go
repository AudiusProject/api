package challenges

import (
	"fmt"
	"testing"
	"time"

	"api.audius.co/database"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPlayCountMilestones_RequiresVerified(t *testing.T) {
	pool := withChallengesDB(t)
	t2025 := time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC)
	database.Seed(pool, database.FixtureMap{
		"blocks": {{"blockhash": "blk_pcm", "number": 1}},
		"users": {
			{"user_id": 400, "wallet": "0x400", "is_verified": true},
			{"user_id": 401, "wallet": "0x401", "is_verified": false},
		},
		"tracks": {
			{"track_id": 4001, "owner_id": 400, "title": "T", "blocknumber": 1},
			{"track_id": 4011, "owner_id": 401, "title": "T", "blocknumber": 1},
		},
		"aggregate_monthly_plays": {
			{"play_item_id": 4001, "timestamp": t2025, "count": 300, "country": ""},
			{"play_item_id": 4011, "timestamp": t2025, "count": 300, "country": ""},
		},
		// plays drive the incremental dirty scan; one per track is enough to
		// mark its owner dirty.
		"plays": {
			{"id": 1, "user_id": 999, "play_item_id": 4001, "created_at": t2025},
			{"id": 2, "user_id": 999, "play_item_id": 4011, "created_at": t2025},
		},
	})

	runProcessor(t, pool, NewPlayCountMilestonesProcessor())

	r1, ok := queryUserChallenge(t, pool, "p1", fmt.Sprintf("%x", 400)+":250")
	require.True(t, ok, "verified user 400 should get a p1 row")
	assert.True(t, r1.IsComplete)
	assert.Equal(t, int32(250), *r1.CurrentStepCount)
	assert.Equal(t, int32(25), r1.Amount)

	_, ok = queryUserChallenge(t, pool, "p1", fmt.Sprintf("%x", 401)+":250")
	assert.False(t, ok, "unverified user gets no row")
}

// A single run reconciles all three tiers; eligibility for each tier is gated on
// the count reaching the previous tier's threshold.
func TestPlayCountMilestones_TierGating(t *testing.T) {
	pool := withChallengesDB(t)
	t2025 := time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC)
	database.Seed(pool, database.FixtureMap{
		"blocks": {{"blockhash": "blk_pcm2", "number": 1}},
		"users": {
			{"user_id": 410, "wallet": "0x410", "is_verified": true}, // count 1500
			{"user_id": 420, "wallet": "0x420", "is_verified": true}, // count 300
		},
		"tracks": {
			{"track_id": 4101, "owner_id": 410, "title": "T", "blocknumber": 1},
			{"track_id": 4201, "owner_id": 420, "title": "T", "blocknumber": 1},
		},
		"aggregate_monthly_plays": {
			{"play_item_id": 4101, "timestamp": t2025, "count": 1500, "country": ""},
			{"play_item_id": 4201, "timestamp": t2025, "count": 300, "country": ""},
		},
		"plays": {
			{"id": 1, "user_id": 999, "play_item_id": 4101, "created_at": t2025},
			{"id": 2, "user_id": 999, "play_item_id": 4201, "created_at": t2025},
		},
	})

	runProcessor(t, pool, NewPlayCountMilestonesProcessor())

	// User 410 (count 1500): p1 + p2 complete, p3 eligible but incomplete.
	r1, ok := queryUserChallenge(t, pool, "p1", fmt.Sprintf("%x", 410)+":250")
	if assert.True(t, ok, "p1 row for 410") {
		assert.True(t, r1.IsComplete)
	}
	r2, ok := queryUserChallenge(t, pool, "p2", fmt.Sprintf("%x", 410)+":1000")
	if assert.True(t, ok, "p2 row for 410") {
		assert.True(t, r2.IsComplete)
	}
	r3, ok := queryUserChallenge(t, pool, "p3", fmt.Sprintf("%x", 410)+":10000")
	if assert.True(t, ok, "p3 row for 410 (eligible at >=1000)") {
		assert.False(t, r3.IsComplete)
		// Reported progress is the raw count (a plays trigger bumps the
		// aggregate by the seeded play, so it lands just over 1500), capped
		// below the 10000 step.
		assert.Greater(t, *r3.CurrentStepCount, int32(1000))
		assert.Less(t, *r3.CurrentStepCount, int32(10000))
	}

	// User 420 (count 300): p1 complete, p2 eligible but incomplete, p3 gated out.
	r1b, ok := queryUserChallenge(t, pool, "p1", fmt.Sprintf("%x", 420)+":250")
	if assert.True(t, ok, "p1 row for 420") {
		assert.True(t, r1b.IsComplete)
	}
	r2b, ok := queryUserChallenge(t, pool, "p2", fmt.Sprintf("%x", 420)+":1000")
	if assert.True(t, ok, "p2 row for 420 (eligible at >=250)") {
		assert.False(t, r2b.IsComplete)
		assert.Greater(t, *r2b.CurrentStepCount, int32(250))
		assert.Less(t, *r2b.CurrentStepCount, int32(1000))
	}
	_, ok = queryUserChallenge(t, pool, "p3", fmt.Sprintf("%x", 420)+":10000")
	assert.False(t, ok, "p3 gated out below 1000 plays")
}
