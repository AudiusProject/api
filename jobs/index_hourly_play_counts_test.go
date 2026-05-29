package jobs

import (
	"context"
	"testing"
	"time"

	"api.audius.co/database"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestHourlyPlayCountsJob_FullPipeline verifies the job buckets new plays
// into the right hour, advances the checkpoint, and is idempotent across
// runs (subsequent runs with no new plays don't change anything).
func TestHourlyPlayCountsJob_FullPipeline(t *testing.T) {
	pool := database.CreateTestDatabase(t, "test_jobs")
	defer pool.Close()

	ctx := context.Background()
	t1 := time.Date(2025, 1, 15, 10, 17, 30, 0, time.UTC) // bucketed to 10:00
	t2 := time.Date(2025, 1, 15, 10, 45, 0, 0, time.UTC)  // same 10:00 bucket
	t3 := time.Date(2025, 1, 15, 11, 5, 0, 0, time.UTC)   // 11:00 bucket

	database.Seed(pool, database.FixtureMap{
		"users": {
			{"user_id": 1, "wallet": "0x01"},
		},
		"tracks": {
			{"track_id": 100, "owner_id": 1, "title": "T"},
		},
		"plays": {
			{"id": 1, "user_id": 1, "play_item_id": 100, "created_at": t1},
			{"id": 2, "user_id": 1, "play_item_id": 100, "created_at": t2},
			{"id": 3, "user_id": 1, "play_item_id": 100, "created_at": t3},
		},
	})

	job := NewHourlyPlayCountsJob(newTestConfig(), pool)

	require.NoError(t, job.run(ctx))

	type row struct {
		Ts    time.Time
		Count int32
	}
	var rows []row
	r, err := pool.Query(ctx, "SELECT hourly_timestamp, play_count FROM hourly_play_counts ORDER BY hourly_timestamp")
	require.NoError(t, err)
	for r.Next() {
		var x row
		require.NoError(t, r.Scan(&x.Ts, &x.Count))
		rows = append(rows, x)
	}
	r.Close()

	require.Len(t, rows, 2, "expected 2 hourly buckets")
	assert.Equal(t, time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC), rows[0].Ts.UTC())
	assert.Equal(t, int32(2), rows[0].Count)
	assert.Equal(t, time.Date(2025, 1, 15, 11, 0, 0, 0, time.UTC), rows[1].Ts.UTC())
	assert.Equal(t, int32(1), rows[1].Count)

	// Checkpoint advanced to max(id).
	cp, err := getCheckpoint(ctx, pool, HourlyPlayCountsCheckpoint)
	require.NoError(t, err)
	assert.Equal(t, int64(3), cp)

	// Re-running with no new plays must be a no-op (counts unchanged).
	require.NoError(t, job.run(ctx))
	var firstBucketCount int32
	require.NoError(t, pool.QueryRow(ctx,
		"SELECT play_count FROM hourly_play_counts WHERE hourly_timestamp = $1",
		time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC),
	).Scan(&firstBucketCount))
	assert.Equal(t, int32(2), firstBucketCount, "second run should not double-count")
}

func TestHourlyPlayCountsJob_NoPlays(t *testing.T) {
	pool := database.CreateTestDatabase(t, "test_jobs")
	defer pool.Close()

	job := NewHourlyPlayCountsJob(newTestConfig(), pool)
	require.NoError(t, job.run(context.Background()))

	var n int
	require.NoError(t, pool.QueryRow(context.Background(),
		"SELECT COUNT(*) FROM hourly_play_counts").Scan(&n))
	assert.Equal(t, 0, n)
}
