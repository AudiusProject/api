package jobs

import (
	"context"
	"testing"
	"time"

	"api.audius.co/database"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestTrendingJob_PopulatesScores is a smoke test against a seeded DB:
// after a run, track_trending_scores has rows for each (type, version,
// time_range) tuple we register. The actual score values follow apps'
// formula verbatim and aren't independently validated here — drift would
// surface in parity comparison against discovery, not in unit tests.
func TestTrendingJob_PopulatesScores(t *testing.T) {
	pool := database.CreateTestDatabase(t, "test_jobs")
	defer pool.Close()
	ctx := context.Background()

	now := time.Now()
	database.Seed(pool, database.FixtureMap{
		"users": {{"user_id": 1, "wallet": "0x01", "name": "Alice"}},
		"tracks": {{
			"track_id": 100, "owner_id": 1, "title": "T",
			"release_date": now.Add(-2 * 24 * time.Hour),
			"created_at":   now.Add(-2 * 24 * time.Hour),
		}},
		"plays": {
			{"id": 1, "user_id": 1, "play_item_id": 100, "created_at": now.Add(-1 * time.Hour)},
		},
	})

	job := NewTrendingJob(newTestConfig(), pool)
	require.NoError(t, job.run(ctx))

	// Track scores: 3 ranges (week, month, allTime) × 2 versions (pnagD,
	// AnlGe) × 2 types (TRACKS, UNDERGROUND_TRACKS via pnagD only) =
	//   2 (week/month + allTime) × 3 + 3 = 9 (TRACKS pnagD: 3, TRACKS AnlGe: 3, UNDERGROUND_TRACKS pnagD: 3)
	// At minimum we expect 9 rows for the seeded track.
	var nTracks int
	require.NoError(t, pool.QueryRow(ctx, "SELECT COUNT(*) FROM track_trending_scores WHERE track_id = 100").Scan(&nTracks))
	assert.GreaterOrEqual(t, nTracks, 9, "expected at least 9 score rows for the seeded track")

	// Re-running should not duplicate (DELETE-then-INSERT per
	// (type, version)).
	require.NoError(t, job.run(ctx))
	var nAfter int
	require.NoError(t, pool.QueryRow(ctx, "SELECT COUNT(*) FROM track_trending_scores WHERE track_id = 100").Scan(&nAfter))
	assert.Equal(t, nTracks, nAfter, "second run must not duplicate rows")
}
