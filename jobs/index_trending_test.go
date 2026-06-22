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
		"users": {
			{"user_id": 1, "wallet": "0x01", "name": "Alice"},
			{
				"user_id": 2, "wallet": "0x02", "name": "Bob",
				"cover_photo": "cover", "profile_picture": "profile", "bio": "bio",
			},
		},
		"aggregate_user": {
			{"user_id": 1, "follower_count": 10},
			{"user_id": 2, "follower_count": 100},
		},
		"tracks": {{
			"track_id": 100, "owner_id": 1, "title": "T",
			"release_date": now.Add(-2 * 24 * time.Hour),
			"created_at":   now.Add(-2 * 24 * time.Hour),
		}},
		"plays": {
			{"id": 1, "user_id": 1, "play_item_id": 100, "created_at": now.Add(-1 * time.Hour)},
		},
		"reposts": {
			{"user_id": 2, "repost_item_id": 100, "repost_type": "track", "created_at": now.Add(-1 * time.Hour)},
		},
	})

	job := NewTrendingJob(newTestConfig(), pool)
	require.NoError(t, job.run(ctx))

	// Track scores: 3 ranges for TRACKS/pnagD, TRACKS/AnlGe, and
	// UNDERGROUND_TRACKS/pnagD. At minimum we expect 9 positive rows for the
	// seeded track.
	var nTracks int
	require.NoError(t, pool.QueryRow(ctx, "SELECT COUNT(*) FROM track_trending_scores WHERE track_id = 100").Scan(&nTracks))
	assert.GreaterOrEqual(t, nTracks, 9, "expected at least 9 score rows for the seeded track")

	// Re-running should keep the same row shape.
	require.NoError(t, job.run(ctx))
	var nAfter int
	require.NoError(t, pool.QueryRow(ctx, "SELECT COUNT(*) FROM track_trending_scores WHERE track_id = 100").Scan(&nAfter))
	assert.Equal(t, nTracks, nAfter, "second run must not duplicate rows")
}

func TestTrendingJob_SkipsZeroTrackScores(t *testing.T) {
	pool := database.CreateTestDatabase(t, "test_jobs")
	defer pool.Close()
	ctx := context.Background()

	now := time.Now()
	database.Seed(pool, database.FixtureMap{
		"users": {{"user_id": 1, "wallet": "0x01", "name": "Alice"}},
		"aggregate_user": {
			{"user_id": 1, "follower_count": 0},
		},
		"tracks": {{
			"track_id": 150, "owner_id": 1, "title": "Quiet Track",
			"release_date": now.Add(-2 * 24 * time.Hour),
			"created_at":   now.Add(-2 * 24 * time.Hour),
		}},
		"plays": {
			{"id": 1, "user_id": 1, "play_item_id": 150, "created_at": now.Add(-1 * time.Hour)},
		},
	})

	job := NewTrendingJob(newTestConfig(), pool)
	require.NoError(t, job.run(ctx))

	var n int
	require.NoError(t, pool.QueryRow(ctx, "SELECT COUNT(*) FROM track_trending_scores WHERE track_id = 150").Scan(&n))
	assert.Equal(t, 0, n, "zero-score tracks should not be persisted")
}

// TestTrendingJob_PrunesStaleRows verifies that a track that drops out of the
// trending source set (here, by being deleted) has its previously-written score
// rows removed on the next run.
func TestTrendingJob_PrunesStaleRows(t *testing.T) {
	pool := database.CreateTestDatabase(t, "test_jobs")
	defer pool.Close()
	ctx := context.Background()

	now := time.Now()
	database.Seed(pool, database.FixtureMap{
		"users": {
			{"user_id": 1, "wallet": "0x01", "name": "Alice"},
			{
				"user_id": 2, "wallet": "0x02", "name": "Bob",
				"cover_photo": "cover", "profile_picture": "profile", "bio": "bio",
			},
		},
		"aggregate_user": {
			{"user_id": 1, "follower_count": 10},
			{"user_id": 2, "follower_count": 100},
		},
		"tracks": {{
			"track_id": 200, "owner_id": 1, "title": "T",
			"release_date": now.Add(-2 * 24 * time.Hour),
			"created_at":   now.Add(-2 * 24 * time.Hour),
		}},
		"plays": {
			{"id": 1, "user_id": 1, "play_item_id": 200, "created_at": now.Add(-1 * time.Hour)},
		},
		"reposts": {
			{"user_id": 2, "repost_item_id": 200, "repost_type": "track", "created_at": now.Add(-1 * time.Hour)},
		},
	})

	job := NewTrendingJob(newTestConfig(), pool)
	require.NoError(t, job.run(ctx))

	var nBefore int
	require.NoError(t, pool.QueryRow(ctx, "SELECT COUNT(*) FROM track_trending_scores WHERE track_id = 200").Scan(&nBefore))
	require.Greater(t, nBefore, 0, "expected score rows for the seeded track before deletion")

	// Remove the track from the trending source set, then re-run. The job must
	// clear its now-stale score rows.
	_, err := pool.Exec(ctx, "UPDATE tracks SET is_delete = true WHERE track_id = 200")
	require.NoError(t, err)
	require.NoError(t, job.run(ctx))

	var nAfter int
	require.NoError(t, pool.QueryRow(ctx, "SELECT COUNT(*) FROM track_trending_scores WHERE track_id = 200").Scan(&nAfter))
	assert.Equal(t, 0, nAfter, "stale score rows must be pruned after the track leaves the source set")
}
