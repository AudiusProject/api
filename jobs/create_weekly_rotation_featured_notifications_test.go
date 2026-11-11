package jobs

import (
	"context"
	"testing"
	"time"

	"api.audius.co/database"
	"api.audius.co/weeklyrotation"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// 2026-09-16 is the Wednesday that opens period 2026-38, so this is the first
// send instant for reporting period 2026-37.
var weeklyRotationFeaturedSendInstant = time.Date(2026, time.September, 16, 18, 0, 0, 0, time.UTC)

func seedWeeklyRotationMix(t *testing.T, pool *pgxpool.Pool, userId int32, year, week int, trackIds []int32) {
	t.Helper()
	_, err := pool.Exec(context.Background(), `
		INSERT INTO weekly_rotation_mixes (user_id, period_start, track_ids)
		VALUES ($1, $2, $3)`,
		userId, weeklyrotation.PeriodStart(year, week), trackIds)
	require.NoError(t, err)
}

func seedWeeklyRotationFeatured(t *testing.T, pool *pgxpool.Pool) {
	database.Seed(pool, database.FixtureMap{
		"users": {
			{"user_id": 1, "handle": "listener1", "wallet": "0x01"},
			{"user_id": 2, "handle": "listener2", "wallet": "0x02"},
			{"user_id": 3, "handle": "listener3", "wallet": "0x03"},
			{"user_id": 10, "handle": "one_track", "wallet": "0x10"},
			{"user_id": 20, "handle": "two_tracks", "wallet": "0x20"},
			{"user_id": 30, "handle": "deactivated", "wallet": "0x30", "is_deactivated": true},
			{"user_id": 40, "handle": "past_shown", "wallet": "0x40"},
			{"user_id": 50, "handle": "deleted_track", "wallet": "0x50"},
			{"user_id": 60, "handle": "other_period", "wallet": "0x60"},
		},
		"tracks": {
			{"track_id": 101, "owner_id": 10, "title": "solo"},
			{"track_id": 201, "owner_id": 20, "title": "a"},
			{"track_id": 202, "owner_id": 20, "title": "b"},
			{"track_id": 301, "owner_id": 30, "title": "gone owner"},
			{"track_id": 401, "owner_id": 40, "title": "slot 31"},
			{"track_id": 501, "owner_id": 50, "title": "deleted", "is_delete": true},
			{"track_id": 601, "owner_id": 60, "title": "this week"},
			{"track_id": 999, "owner_id": 30, "title": "filler"},
		},
	})

	// Listener 3: 201 in slot 1, filler (deactivated owner) up to the last
	// shown slot, then 401 in the first slot clients don't show.
	listener3 := []int32{201}
	for len(listener3) < weeklyrotation.ShownSize {
		listener3 = append(listener3, 999)
	}
	listener3 = append(listener3, 401)

	// Period 2026-37 (the one being reported).
	seedWeeklyRotationMix(t, pool, 1, 2026, 37, []int32{101, 201, 301, 501})
	seedWeeklyRotationMix(t, pool, 2, 2026, 37, []int32{202, 101})
	seedWeeklyRotationMix(t, pool, 3, 2026, 37, listener3)
	// Period 2026-38 (in progress, not reported yet).
	seedWeeklyRotationMix(t, pool, 1, 2026, 38, []int32{601})
}

func newWeeklyRotationFeaturedJob(pool database.DbPool, now time.Time) *WeeklyRotationFeaturedNotificationsJob {
	job := NewWeeklyRotationFeaturedNotificationsJob(newTestConfig(), pool)
	job.now = func() time.Time { return now }
	return job
}

func TestWeeklyRotationFeaturedNotifications_FanOut(t *testing.T) {
	pool := database.CreateTestDatabase(t, "test_jobs")
	defer pool.Close()
	ctx := context.Background()
	seedWeeklyRotationFeatured(t, pool)

	now := weeklyRotationFeaturedSendInstant.Add(30 * time.Minute)
	job := newWeeklyRotationFeaturedJob(pool, now)
	require.NoError(t, job.run(ctx))

	rows, err := pool.Query(ctx, `
		SELECT specifier, group_id, user_ids, data, timestamp
		FROM notification WHERE type = 'weekly_rotation_featured' ORDER BY specifier::int`)
	require.NoError(t, err)
	defer rows.Close()

	type row struct {
		specifier string
		groupID   string
		userIDs   []int32
		data      map[string]any
		ts        time.Time
	}
	var got []row
	for rows.Next() {
		var r row
		require.NoError(t, rows.Scan(&r.specifier, &r.groupID, &r.userIDs, &r.data, &r.ts))
		got = append(got, r)
	}
	// Owners 30 (deactivated), 40 (only past the shown slots), 50 (deleted
	// track) and 60 (other period) get nothing.
	require.Len(t, got, 2)

	// One track in two listeners' rotations.
	assert.Equal(t, "10", got[0].specifier)
	assert.Equal(t, "weekly_rotation_featured:2026-37:10", got[0].groupID)
	assert.Equal(t, []int32{10}, got[0].userIDs)
	assert.Equal(t, map[string]any{
		"year": float64(2026), "week": float64(37),
		"track_id": float64(101), "track_count": float64(1), "rotation_count": float64(2),
	}, got[0].data)
	assert.WithinDuration(t, now, got[0].ts, time.Second)

	// Two tracks across three rotations; 201 was in two, so it leads.
	assert.Equal(t, "20", got[1].specifier)
	assert.Equal(t, map[string]any{
		"year": float64(2026), "week": float64(37),
		"track_id": float64(201), "track_count": float64(2), "rotation_count": float64(3),
	}, got[1].data)

	// Idempotent: the same job is drained, and a restart adds nothing.
	assert.True(t, job.drained)
	require.NoError(t, job.run(ctx))
	fresh := newWeeklyRotationFeaturedJob(pool, now)
	require.NoError(t, fresh.run(ctx))
	assert.Equal(t, 2, countNotifications(t, ctx, pool, "weekly_rotation_featured"))

	// The next rollover reports 2026-38.
	nextWeek := newWeeklyRotationFeaturedJob(pool, now.AddDate(0, 0, 7))
	require.NoError(t, nextWeek.run(ctx))
	assert.Equal(t, 3, countNotifications(t, ctx, pool, "weekly_rotation_featured"))
	var groupID string
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT group_id FROM notification WHERE group_id LIKE 'weekly_rotation_featured:2026-38:%'`).Scan(&groupID))
	assert.Equal(t, "weekly_rotation_featured:2026-38:60", groupID)
}

func TestWeeklyRotationFeaturedNotifications_SendWindow(t *testing.T) {
	pool := database.CreateTestDatabase(t, "test_jobs")
	defer pool.Close()
	ctx := context.Background()
	seedWeeklyRotationFeatured(t, pool)

	cases := []struct {
		name string
		now  time.Time
		sent bool
	}{
		{"rollover instant, before send hour", weeklyRotationFeaturedSendInstant.Add(-18 * time.Hour), false},
		{"one second before send hour", weeklyRotationFeaturedSendInstant.Add(-time.Second), false},
		{"send hour", weeklyRotationFeaturedSendInstant, true},
		{"one second before the window closes", weeklyRotationFeaturedSendInstant.Add(24*time.Hour - time.Second), true},
		{"window closed (Thursday 18:00)", weeklyRotationFeaturedSendInstant.Add(24 * time.Hour), false},
		{"the following Monday", weeklyRotationFeaturedSendInstant.AddDate(0, 0, 5), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := pool.Exec(ctx, `DELETE FROM notification WHERE type = 'weekly_rotation_featured'`)
			require.NoError(t, err)

			job := newWeeklyRotationFeaturedJob(pool, tc.now)
			require.NoError(t, job.run(ctx))
			want := 0
			if tc.sent {
				want = 2
			}
			assert.Equal(t, want, countNotifications(t, ctx, pool, "weekly_rotation_featured"))
		})
	}
}

func TestWeeklyRotationFeaturedNotifications_StageSendsAllWeek(t *testing.T) {
	pool := database.CreateTestDatabase(t, "test_jobs")
	defer pool.Close()
	ctx := context.Background()
	seedWeeklyRotationFeatured(t, pool)

	// Rollover instant itself, well before the production send hour.
	job := newWeeklyRotationFeaturedJob(pool, weeklyRotationFeaturedSendInstant.Add(-18*time.Hour))
	job.env = "stage"
	require.NoError(t, job.run(ctx))
	assert.Equal(t, 2, countNotifications(t, ctx, pool, "weekly_rotation_featured"))
}

func TestWeeklyRotationFeaturedNotifications_Pacing(t *testing.T) {
	pool := database.CreateTestDatabase(t, "test_jobs")
	defer pool.Close()
	ctx := context.Background()
	seedWeeklyRotationFeatured(t, pool)

	job := newWeeklyRotationFeaturedJob(pool, weeklyRotationFeaturedSendInstant)
	job.batchSize = 1

	require.NoError(t, job.run(ctx))
	assert.Equal(t, 1, countNotifications(t, ctx, pool, "weekly_rotation_featured"))
	assert.Equal(t, int32(10), job.cursorOwnerId)
	assert.False(t, job.drained)

	require.NoError(t, job.run(ctx))
	assert.Equal(t, 2, countNotifications(t, ctx, pool, "weekly_rotation_featured"))
	assert.Equal(t, int32(20), job.cursorOwnerId)

	// The next run comes back empty and marks the period drained.
	require.NoError(t, job.run(ctx))
	assert.Equal(t, 2, countNotifications(t, ctx, pool, "weekly_rotation_featured"))
	assert.True(t, job.drained)
}
