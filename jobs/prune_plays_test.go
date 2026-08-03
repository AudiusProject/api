package jobs

import (
	"context"
	"testing"
	"time"

	"api.audius.co/database"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPrunePlaysJob_DeletesOnlyOld(t *testing.T) {
	pool := database.CreateTestDatabase(t, "test_jobs")
	defer pool.Close()
	ctx := context.Background()

	old1 := time.Now().Add(-500 * 24 * time.Hour)
	old2 := time.Now().Add(-450 * 24 * time.Hour)
	recent := time.Now().Add(-1 * time.Hour)

	database.Seed(pool, database.FixtureMap{
		"users": {
			{"user_id": 1, "wallet": "0x01"},
		},
		"tracks": {
			{"track_id": 100, "owner_id": 1, "title": "T"},
		},
		"plays": {
			{"id": 1, "user_id": 1, "play_item_id": 100, "created_at": old1},
			{"id": 2, "user_id": 1, "play_item_id": 100, "created_at": old2},
			{"id": 3, "user_id": 1, "play_item_id": 100, "created_at": recent},
		},
	})

	job := NewPrunePlaysJob(newTestConfig(), pool).
		WithAge(400 * 24 * time.Hour).
		WithBatchSize(100)

	require.NoError(t, job.run(ctx))

	var remaining []int64
	r, err := pool.Query(ctx, "SELECT id FROM plays ORDER BY id")
	require.NoError(t, err)
	for r.Next() {
		var id int64
		require.NoError(t, r.Scan(&id))
		remaining = append(remaining, id)
	}
	r.Close()

	assert.Equal(t, []int64{3}, remaining, "only the recent play should remain")
}

func TestPrunePlaysJob_RespectsBatchSize(t *testing.T) {
	pool := database.CreateTestDatabase(t, "test_jobs")
	defer pool.Close()
	ctx := context.Background()

	old := time.Now().Add(-500 * 24 * time.Hour)
	database.Seed(pool, database.FixtureMap{
		"users":  {{"user_id": 1, "wallet": "0x01"}},
		"tracks": {{"track_id": 100, "owner_id": 1, "title": "T"}},
		"plays": {
			{"id": 1, "user_id": 1, "play_item_id": 100, "created_at": old},
			{"id": 2, "user_id": 1, "play_item_id": 100, "created_at": old.Add(time.Second)},
			{"id": 3, "user_id": 1, "play_item_id": 100, "created_at": old.Add(2 * time.Second)},
		},
	})

	job := NewPrunePlaysJob(newTestConfig(), pool).
		WithAge(400 * 24 * time.Hour).
		WithBatchSize(2)

	require.NoError(t, job.run(ctx))

	var n int
	require.NoError(t, pool.QueryRow(ctx, "SELECT COUNT(*) FROM plays").Scan(&n))
	assert.Equal(t, 1, n, "1 row should remain after deleting 2")
}
