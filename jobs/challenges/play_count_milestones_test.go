package challenges

import (
	"fmt"
	"testing"
	"time"

	"api.audius.co/database"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPlayCount250_RequiresVerified(t *testing.T) {
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
	})

	runProcessor(t, pool, NewPlayCount250Processor())

	r1, ok := queryUserChallenge(t, pool, "p1", fmt.Sprintf("%x", 400)+":250")
	require.True(t, ok, "verified user 400 should get a p1 row")
	assert.True(t, r1.IsComplete)
	assert.Equal(t, int32(250), *r1.CurrentStepCount)
	assert.Equal(t, int32(25), r1.Amount)

	_, ok = queryUserChallenge(t, pool, "p1", fmt.Sprintf("%x", 401)+":250")
	assert.False(t, ok, "unverified user gets no row")
}

func TestPlayCount1000_GatedOnPrevious(t *testing.T) {
	pool := withChallengesDB(t)
	t2025 := time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC)
	database.Seed(pool, database.FixtureMap{
		"blocks": {{"blockhash": "blk_pcm2", "number": 1}},
		"users":  {{"user_id": 410, "wallet": "0x410", "is_verified": true}},
		"tracks": {{"track_id": 4101, "owner_id": 410, "title": "T", "blocknumber": 1}},
		"aggregate_monthly_plays": {
			{"play_item_id": 4101, "timestamp": t2025, "count": 1500, "country": ""},
		},
	})

	// Without p1 completed, p2 should not create a row.
	runProcessor(t, pool, NewPlayCount1000Processor())
	_, ok := queryUserChallenge(t, pool, "p2", fmt.Sprintf("%x", 410)+":1000")
	assert.False(t, ok, "p2 gated on p1 completion")

	// Complete p1 first.
	runProcessor(t, pool, NewPlayCount250Processor())
	// Now p2 should land.
	runProcessor(t, pool, NewPlayCount1000Processor())
	r, ok := queryUserChallenge(t, pool, "p2", fmt.Sprintf("%x", 410)+":1000")
	if assert.True(t, ok) {
		assert.True(t, r.IsComplete)
	}
}
