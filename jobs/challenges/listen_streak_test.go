package challenges

import (
	"context"
	"fmt"
	"testing"
	"time"

	"api.audius.co/database"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestListenStreak_SkippedWhenInactive — challenge "e" is active=false in
// challenges.json by default. Until/unless apps enables it, the processor
// is a no-op. This test pins that behavior.
func TestListenStreak_SkippedWhenInactive(t *testing.T) {
	pool := withChallengesDB(t)
	database.Seed(pool, database.FixtureMap{
		"blocks": {{"blockhash": "blk_ls", "number": 1}},
		"users":  {{"user_id": 600, "wallet": "0x600"}},
		"tracks": {{"track_id": 6000, "owner_id": 600, "title": "T", "blocknumber": 1}},
		"plays": {
			{"id": 1, "user_id": 600, "play_item_id": 6000, "created_at": time.Now()},
		},
	})

	runProcessor(t, pool, &ListenStreakProcessor{})

	// Nothing in challenge_listen_streak because the catalog row is inactive.
	var n int
	require.NoError(t, pool.QueryRow(context.Background(),
		"SELECT COUNT(*) FROM challenge_listen_streak").Scan(&n))
	assert.Equal(t, 0, n)
}

// TestListenStreak_AdvancesAcrossDays — exercises the active path by
// activating the catalog row, then feeding plays at increasing day
// boundaries. Expects streak to count up.
func TestListenStreak_AdvancesAcrossDays(t *testing.T) {
	pool := withChallengesDB(t)
	ctx := context.Background()
	_, err := pool.Exec(ctx, "UPDATE challenges SET active = true WHERE id = 'e'")
	require.NoError(t, err)

	day := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
	database.Seed(pool, database.FixtureMap{
		"blocks": {{"blockhash": "blk_ls", "number": 1}},
		"users":  {{"user_id": 610, "wallet": "0x610"}},
		"tracks": {{"track_id": 6100, "owner_id": 610, "title": "T", "blocknumber": 1}},
		"plays": {
			{"id": 10, "user_id": 610, "play_item_id": 6100, "created_at": day},
			{"id": 11, "user_id": 610, "play_item_id": 6100, "created_at": day.Add(20 * time.Hour)}, // streak = 2
			{"id": 12, "user_id": 610, "play_item_id": 6100, "created_at": day.Add(40 * time.Hour)}, // streak = 3
		},
	})

	runProcessor(t, pool, &ListenStreakProcessor{})

	var streak int32
	require.NoError(t, pool.QueryRow(ctx,
		"SELECT listen_streak FROM challenge_listen_streak WHERE user_id = 610").Scan(&streak))
	assert.Equal(t, int32(3), streak, "3 plays at >16h gaps → streak = 3")

	// Should have written 3 user_challenge rows (one per advance), each
	// with specifier {hex_uid}{YYYYMMDDHH}.
	for _, ts := range []time.Time{day, day.Add(20 * time.Hour), day.Add(40 * time.Hour)} {
		spec := fmt.Sprintf("%x%s", 610, ts.UTC().Format("2006010215"))
		_, ok := queryUserChallenge(t, pool, "e", spec)
		assert.True(t, ok, "expected user_challenge row for advance at %s", ts)
	}
}
