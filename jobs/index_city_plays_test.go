package jobs

import (
	"context"
	"testing"
	"time"

	"api.audius.co/database"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCityPlaysJob(t *testing.T) {
	pool := database.CreateTestDatabase(t, "test_jobs")
	defer pool.Close()
	ctx := context.Background()

	day1 := time.Date(2026, 11, 2, 10, 0, 0, 0, time.UTC)
	day2 := time.Date(2026, 11, 3, 23, 30, 0, 0, time.UTC)

	database.Seed(pool, database.FixtureMap{
		"users": {
			{"user_id": 1, "wallet": "0x01"},
			{"user_id": 2, "wallet": "0x02"},
		},
		"tracks": {
			{"track_id": 100, "owner_id": 1, "title": "T"},
		},
		"plays": {
			{"id": 1, "user_id": 2, "play_item_id": 100, "created_at": day1, "city": "Atlanta", "region": "Georgia", "country": "United States"},
			{"id": 2, "user_id": nil, "play_item_id": 100, "created_at": day1, "city": " Atlanta ", "region": "Georgia", "country": "United States"},
			// The owner's own play is skipped.
			{"id": 3, "user_id": 1, "play_item_id": 100, "created_at": day1, "city": "Atlanta", "region": "Georgia", "country": "United States"},
			// No city: skipped.
			{"id": 4, "user_id": 2, "play_item_id": 100, "created_at": day1, "city": "", "country": "United States"},
			{"id": 5, "user_id": 2, "play_item_id": 100, "created_at": day1, "city": nil},
			{"id": 6, "user_id": 2, "play_item_id": 100, "created_at": day2, "city": "Atlanta", "region": "Georgia", "country": "United States"},
			{"id": 7, "user_id": 2, "play_item_id": 100, "created_at": day2, "city": "Berlin", "region": "Land Berlin", "country": "Germany"},
		},
	})

	// Batches of 3 ids: three runs to catch up, a fourth is a no-op.
	job := NewCityPlaysJob(newTestConfig(), pool).WithBatchSize(3)
	for range 4 {
		require.NoError(t, job.run(ctx))
	}

	type row struct {
		Day   string
		City  string
		Count int32
	}
	var rows []row
	r, err := pool.Query(ctx, `
		SELECT day::text, city, count FROM aggregate_daily_city_plays
		WHERE play_item_id = 100 ORDER BY day, city`)
	require.NoError(t, err)
	for r.Next() {
		var x row
		require.NoError(t, r.Scan(&x.Day, &x.City, &x.Count))
		rows = append(rows, x)
	}
	r.Close()

	assert.Equal(t, []row{
		{"2026-11-02", "Atlanta", 2},
		{"2026-11-03", "Atlanta", 1},
		{"2026-11-03", "Berlin", 1},
	}, rows)

	cp, err := getCheckpoint(ctx, pool, CityPlaysCheckpoint)
	require.NoError(t, err)
	assert.Equal(t, int64(7), cp)
}

func TestCityPlaysJob_SkipsPrunedIds(t *testing.T) {
	pool := database.CreateTestDatabase(t, "test_jobs")
	defer pool.Close()
	ctx := context.Background()

	database.Seed(pool, database.FixtureMap{
		"users":  {{"user_id": 1, "wallet": "0x01"}},
		"tracks": {{"track_id": 100, "owner_id": 1, "title": "T"}},
		"plays": {
			{"id": 1_000_000, "play_item_id": 100, "created_at": time.Now(), "city": "Lagos", "region": "Lagos", "country": "Nigeria"},
		},
	})

	job := NewCityPlaysJob(newTestConfig(), pool).WithBatchSize(10)
	require.NoError(t, job.run(ctx))

	var count int32
	require.NoError(t, pool.QueryRow(ctx, "SELECT SUM(count) FROM aggregate_daily_city_plays").Scan(&count))
	assert.Equal(t, int32(1), count)
}
