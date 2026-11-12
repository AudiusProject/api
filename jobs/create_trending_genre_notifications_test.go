package jobs

import (
	"context"
	"testing"
	"time"

	"api.audius.co/database"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// 2026-09-18 is a Friday.
var trendingGenreFriday = time.Date(2026, time.September, 18, 0, 0, 0, 0, time.UTC)

// seedTrendingGenre seeds three genres:
//   - Dubstep: 12 tracks (1001-1012), scores 120 down to 10. Qualifies.
//   - Blues: 9 tracks (2001-2009). Too few, no notifications.
//   - Electronic: 10 tracks (3001-3010). Exactly the minimum, qualifies.
//
// Inside Dubstep, 1002 already got an overall trending notification today,
// 1003 is unlisted (not on the chart, so 1004+ move up a rank) and 1005's
// owner is deactivated.
func seedTrendingGenre(t *testing.T, pool *pgxpool.Pool) {
	users := []map[string]any{
		{"user_id": 1, "handle": "artist1", "wallet": "0x01"},
		{"user_id": 2, "handle": "deactivated", "wallet": "0x02", "is_deactivated": true},
	}
	var tracks, scores []map[string]any
	add := func(id int, genre string, score float64) {
		owner := 1
		if id == 1005 {
			owner = 2
		}
		track := map[string]any{"track_id": id, "owner_id": owner, "title": "t", "genre": genre}
		if id == 1003 {
			track["is_unlisted"] = true
		}
		tracks = append(tracks, track)
		scores = append(scores, map[string]any{
			"track_id": id, "genre": genre, "time_range": "week", "score": score,
		})
	}
	for i := 1; i <= 12; i++ {
		add(1000+i, "Dubstep", float64(130-10*i))
	}
	for i := 1; i <= 9; i++ {
		add(2000+i, "Blues", float64(100-i))
	}
	for i := 1; i <= 10; i++ {
		add(3000+i, "Electronic", float64(100-i))
	}
	// A month score and a stale genre-less row never count.
	scores = append(scores,
		map[string]any{"track_id": 1001, "genre": "Dubstep", "time_range": "month", "score": 999.0},
		map[string]any{"track_id": 4001, "genre": nil, "time_range": "week", "score": 999.0},
	)
	tracks = append(tracks, map[string]any{"track_id": 4001, "owner_id": 1, "title": "old"})

	database.Seed(pool, database.FixtureMap{
		"users":                 users,
		"tracks":                tracks,
		"track_trending_scores": scores,
		"notification": {
			{
				"specifier": "1002", "group_id": "trending:time_range:week:genre:all:rank:3:track_id:1002:timestamp:1",
				"type": "trending", "user_ids": []int32{1}, "data": map[string]any{"track_id": 1002},
				"timestamp": trendingGenreFriday.Add(time.Hour),
			},
		},
	})
}

func newTrendingGenreJob(pool database.DbPool, now time.Time) *TrendingGenreNotificationsJob {
	job := NewTrendingGenreNotificationsJob(newTestConfig(), pool)
	job.now = func() time.Time { return now }
	return job
}

func TestTrendingGenreNotifications_Snapshot(t *testing.T) {
	pool := database.CreateTestDatabase(t, "test_jobs")
	defer pool.Close()
	ctx := context.Background()
	seedTrendingGenre(t, pool)

	now := trendingGenreFriday.Add(18*time.Hour + time.Minute)
	require.NoError(t, newTrendingGenreJob(pool, now).run(ctx))

	rows, err := pool.Query(ctx, `
		SELECT specifier, group_id, user_ids, data
		FROM notification WHERE type = 'trending_genre'
		ORDER BY (data->>'genre'), (data->>'rank')::int`)
	require.NoError(t, err)
	defer rows.Close()

	type row struct {
		specifier string
		groupID   string
		userIDs   []int32
		data      map[string]any
	}
	var got []row
	for rows.Next() {
		var r row
		require.NoError(t, rows.Scan(&r.specifier, &r.groupID, &r.userIDs, &r.data))
		got = append(got, r)
	}

	byGenre := map[string][]string{}
	for _, r := range got {
		byGenre[r.data["genre"].(string)] = append(byGenre[r.data["genre"].(string)], r.specifier)
	}
	// Dubstep chart after dropping 1003 (unlisted) and 1005 (deactivated
	// owner): 1001, 1002, 1004, 1006..1012 = 10 tracks, all top 10. 1002 is
	// skipped because it already got an overall trending notification.
	assert.Equal(t, []string{"1001", "1004", "1006", "1007", "1008", "1009", "1010", "1011", "1012"}, byGenre["Dubstep"])
	assert.Len(t, byGenre["Electronic"], 10)
	assert.NotContains(t, byGenre, "Blues")

	first := got[0]
	assert.Equal(t, "1001", first.specifier)
	assert.Equal(t, "trending_genre:2026-09-18:1001", first.groupID)
	assert.Equal(t, []int32{1}, first.userIDs)
	assert.Equal(t, map[string]any{
		"track_id": float64(1001),
		"genre":    "Dubstep",
		"rank":     float64(1),
		"week":     "2026-09-18",
	}, first.data)
	// 1004 moves up to #3 once 1003 is off the chart.
	assert.Equal(t, float64(3), got[1].data["rank"])
}

func TestTrendingGenreNotifications_OncePerWeek(t *testing.T) {
	pool := database.CreateTestDatabase(t, "test_jobs")
	defer pool.Close()
	ctx := context.Background()
	seedTrendingGenre(t, pool)

	count := func() int {
		var n int
		require.NoError(t, pool.QueryRow(ctx, `SELECT COUNT(*) FROM notification WHERE type = 'trending_genre'`).Scan(&n))
		return n
	}

	now := trendingGenreFriday.Add(19 * time.Hour)
	require.NoError(t, newTrendingGenreJob(pool, now).run(ctx))
	before := count()
	require.Equal(t, 19, before)

	// Scores move later in the day; a fresh replica must not add a second
	// snapshot for the same week.
	_, err := pool.Exec(ctx, `UPDATE track_trending_scores SET score = 1000 WHERE track_id = 1003`)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `UPDATE tracks SET is_unlisted = false WHERE track_id = 1003`)
	require.NoError(t, err)
	require.NoError(t, newTrendingGenreJob(pool, now.Add(3*time.Hour)).run(ctx))
	assert.Equal(t, before, count())

	// Next Friday is a new snapshot.
	require.NoError(t, newTrendingGenreJob(pool, now.AddDate(0, 0, 7)).run(ctx))
	assert.Greater(t, count(), before)
}

func TestTrendingGenreNotifications_SendWindow(t *testing.T) {
	pool := database.CreateTestDatabase(t, "test_jobs")
	defer pool.Close()
	ctx := context.Background()
	seedTrendingGenre(t, pool)

	count := func() int {
		var n int
		require.NoError(t, pool.QueryRow(ctx, `SELECT COUNT(*) FROM notification WHERE type = 'trending_genre'`).Scan(&n))
		return n
	}

	// Thursday, Friday morning and Saturday after 18:00 are all outside.
	for _, now := range []time.Time{
		trendingGenreFriday.Add(-time.Hour),
		trendingGenreFriday.Add(17*time.Hour + 59*time.Minute),
		trendingGenreFriday.Add(42 * time.Hour),
	} {
		require.NoError(t, newTrendingGenreJob(pool, now).run(ctx))
		assert.Equal(t, 0, count(), now)
	}
	// Saturday before 18:00 is still inside.
	require.NoError(t, newTrendingGenreJob(pool, trendingGenreFriday.Add(41*time.Hour)).run(ctx))
	assert.Equal(t, 19, count())
}

func TestTrendingGenreNotifications_Stage(t *testing.T) {
	job := NewTrendingGenreNotificationsJob(newTestConfig(), nil)
	job.env = "stage"
	// A Tuesday maps to the previous Friday.
	friday, ok := job.sendWindow(time.Date(2026, time.September, 22, 3, 0, 0, 0, time.UTC))
	assert.True(t, ok)
	assert.Equal(t, trendingGenreFriday, friday)
}
