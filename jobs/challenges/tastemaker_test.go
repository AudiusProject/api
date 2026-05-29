package challenges

import (
	"context"
	"fmt"
	"testing"

	"api.audius.co/database"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestTastemaker_EarliestRepostersAndSavers — for a track in the top-10
// trending, the earliest 10 reposters AND earliest 10 savers each earn a
// tastemaker row (deduped by specifier).
func TestTastemaker_EarliestRepostersAndSavers(t *testing.T) {
	pool := withChallengesDB(t)
	ctx := context.Background()

	database.Seed(pool, database.FixtureMap{
		"blocks": {{"blockhash": "blk_tm", "number": 1}},
		"users": {
			{"user_id": 1200, "wallet": "0x1200"},
			{"user_id": 1201, "wallet": "0x1201"},
			{"user_id": 1202, "wallet": "0x1202"},
		},
		"tracks": {{"track_id": 120000, "owner_id": 1200, "title": "Hit", "blocknumber": 1}},
	})

	// Put the track in track_trending_scores (top of week range).
	_, err := pool.Exec(ctx, `
		INSERT INTO track_trending_scores (track_id, type, version, time_range, score, created_at)
		VALUES (120000, 'TRACKS', 'pnagD', 'week', 999.0, now())
	`)
	require.NoError(t, err)

	// 1201 reposts, 1202 saves.
	_, err = pool.Exec(ctx, `
		INSERT INTO reposts (user_id, repost_item_id, repost_type, is_current, is_delete, created_at, txhash)
		VALUES (1201, 120000, 'track', true, false, now() - interval '1 hour', 'tx-rp')
	`)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `
		INSERT INTO saves (user_id, save_item_id, save_type, is_current, is_delete, created_at, txhash)
		VALUES (1202, 120000, 'track', true, false, now() - interval '30 minutes', 'tx-sv')
	`)
	require.NoError(t, err)

	runProcessor(t, pool, &TastemakerProcessor{})

	for _, uid := range []int{1201, 1202} {
		spec := fmt.Sprintf("%x:t:%x", uid, 120000)
		r, ok := queryUserChallenge(t, pool, "t", spec)
		if assert.True(t, ok, "user %d should have tastemaker row", uid) {
			assert.True(t, r.IsComplete)
			assert.Equal(t, int32(100), r.Amount)
		}
	}
}
