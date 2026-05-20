package jobs

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"api.audius.co/database"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUserListeningHistoryJob_InsertsFirstHistory(t *testing.T) {
	pool := database.CreateTestDatabase(t, "test_jobs")
	defer pool.Close()
	ctx := context.Background()

	t1 := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	t2 := time.Date(2025, 1, 15, 11, 0, 0, 0, time.UTC)

	database.Seed(pool, database.FixtureMap{
		"users":  {{"user_id": 1, "wallet": "0x01"}},
		"tracks": {{"track_id": 100, "owner_id": 1, "title": "T"}, {"track_id": 101, "owner_id": 1, "title": "T2"}},
		"plays": {
			{"id": 1, "user_id": 1, "play_item_id": 100, "created_at": t1},
			{"id": 2, "user_id": 1, "play_item_id": 100, "created_at": t2}, // 2 plays of track 100
			{"id": 3, "user_id": 1, "play_item_id": 101, "created_at": t1}, // 1 play of track 101
		},
	})

	job := NewUserListeningHistoryJob(newTestConfig(), pool)
	require.NoError(t, job.run(ctx))

	var raw []byte
	require.NoError(t, pool.QueryRow(ctx,
		"SELECT listening_history FROM user_listening_history WHERE user_id = 1").Scan(&raw))
	var entries []listenEntry
	require.NoError(t, json.Unmarshal(raw, &entries))

	require.Len(t, entries, 2)
	// Sorted desc by timestamp — track 100 last played at t2.
	assert.Equal(t, int64(100), entries[0].TrackID)
	assert.Equal(t, int64(2), entries[0].PlayCount)
	assert.Equal(t, int64(101), entries[1].TrackID)
	assert.Equal(t, int64(1), entries[1].PlayCount)
}

func TestUserListeningHistoryJob_MergesAcrossRuns(t *testing.T) {
	pool := database.CreateTestDatabase(t, "test_jobs")
	defer pool.Close()
	ctx := context.Background()

	t1 := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	t2 := time.Date(2025, 1, 15, 11, 0, 0, 0, time.UTC)

	database.Seed(pool, database.FixtureMap{
		"users":  {{"user_id": 1, "wallet": "0x01"}},
		"tracks": {{"track_id": 100, "owner_id": 1, "title": "T"}},
		"plays": {
			{"id": 1, "user_id": 1, "play_item_id": 100, "created_at": t1},
		},
	})

	job := NewUserListeningHistoryJob(newTestConfig(), pool)
	require.NoError(t, job.run(ctx))

	// Insert a new play with id past the checkpoint.
	_, err := pool.Exec(ctx,
		"INSERT INTO plays (id, user_id, play_item_id, created_at) VALUES (2, 1, 100, $1)", t2)
	require.NoError(t, err)

	require.NoError(t, job.run(ctx))

	var raw []byte
	require.NoError(t, pool.QueryRow(ctx,
		"SELECT listening_history FROM user_listening_history WHERE user_id = 1").Scan(&raw))
	var entries []listenEntry
	require.NoError(t, json.Unmarshal(raw, &entries))

	require.Len(t, entries, 1)
	assert.Equal(t, int64(100), entries[0].TrackID)
	assert.Equal(t, int64(2), entries[0].PlayCount, "play_count should sum across runs")
}

func TestMergeListeningHistory_CapsToLimit(t *testing.T) {
	now := time.Now()
	plays := make([]rawPlay, 0, 50)
	for i := 0; i < 50; i++ {
		plays = append(plays, rawPlay{
			TrackID:   int64(1000 + i),
			CreatedAt: now.Add(time.Duration(i) * time.Minute),
		})
	}
	merged := mergeListeningHistory(nil, plays, 10)
	assert.Len(t, merged, 10)
	// Newest plays first.
	assert.Equal(t, int64(1049), merged[0].TrackID)
}
