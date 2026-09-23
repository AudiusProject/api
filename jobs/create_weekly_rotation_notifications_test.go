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

// 2026-09-09 is a Wednesday, so this is the first send instant of the
// 2026-37 period (16:00 UTC on rollover day).
var weeklyRotationSendInstant = time.Date(2026, time.September, 9, 16, 0, 0, 0, time.UTC)

func seedWeeklyRotationListeners(pool *pgxpool.Pool, now time.Time) {
	database.Seed(pool, database.FixtureMap{
		"users": {
			{"user_id": 1, "handle": "one", "wallet": "0x01"},
			{"user_id": 2, "handle": "two", "wallet": "0x02"},
			{"user_id": 3, "handle": "three", "wallet": "0x03", "is_deactivated": true},
			{"user_id": 4, "handle": "four", "wallet": "0x04"},
			{"user_id": 5, "handle": "five", "wallet": "0x05"},
		},
		"plays": {
			// Listened this week -> notified.
			{"id": 1, "user_id": 1, "play_item_id": 100, "created_at": now.Add(-2 * 24 * time.Hour)},
			// Last listen too long ago -> not notified.
			{"id": 2, "user_id": 2, "play_item_id": 100, "created_at": now.Add(-40 * 24 * time.Hour)},
			// Deactivated -> not notified even though active.
			{"id": 3, "user_id": 3, "play_item_id": 100, "created_at": now.Add(-24 * time.Hour)},
			// Listened near the edge of the window -> notified.
			{"id": 4, "user_id": 4, "play_item_id": 100, "created_at": now.Add(-29 * 24 * time.Hour)},
			// Anonymous play -> no one to notify.
			{"id": 5, "user_id": nil, "play_item_id": 100, "created_at": now.Add(-time.Hour)},
			// user 5 has never listened -> not notified.
		},
	})
}

func newWeeklyRotationJob(pool database.DbPool, now time.Time) *WeeklyRotationNotificationsJob {
	job := NewWeeklyRotationNotificationsJob(newTestConfig(), pool)
	job.now = func() time.Time { return now }
	return job
}

func TestWeeklyRotationNotifications_FanOut(t *testing.T) {
	pool := database.CreateTestDatabase(t, "test_jobs")
	defer pool.Close()
	ctx := context.Background()

	now := weeklyRotationSendInstant.Add(30 * time.Minute)
	seedWeeklyRotationListeners(pool, now)

	job := newWeeklyRotationJob(pool, now)
	require.NoError(t, job.run(ctx))

	rows, err := pool.Query(ctx, `
		SELECT specifier, group_id, user_ids, data, timestamp
		FROM notification WHERE type = 'weekly_rotation' ORDER BY specifier`)
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
	require.Len(t, got, 2, "only live users who listened inside the window")

	assert.Equal(t, "1", got[0].specifier)
	assert.Equal(t, "weekly_rotation:2026-37:1", got[0].groupID)
	assert.Equal(t, []int32{1}, got[0].userIDs)
	assert.Equal(t, map[string]any{"year": float64(2026), "week": float64(37)}, got[0].data)
	assert.WithinDuration(t, now, got[0].ts, time.Second)

	assert.Equal(t, "4", got[1].specifier)
	assert.Equal(t, "weekly_rotation:2026-37:4", got[1].groupID)

	// Idempotent within the period: a second run adds nothing, and the
	// cursor has moved past everyone.
	require.NoError(t, job.run(ctx))
	assert.Equal(t, 2, countNotifications(t, ctx, pool, "weekly_rotation"))
	assert.Equal(t, int32(4), job.cursorUserId)

	// A restart (fresh cursor) still adds nothing: the NOT EXISTS covers it.
	fresh := newWeeklyRotationJob(pool, now)
	require.NoError(t, fresh.run(ctx))
	assert.Equal(t, 2, countNotifications(t, ctx, pool, "weekly_rotation"))

	// Next period: a new row under the new group id for user 1. User 4's
	// last listen (29 days before this period's send) has aged out of the
	// 30-day window by then.
	nextWeek := newWeeklyRotationJob(pool, now.AddDate(0, 0, 7))
	require.NoError(t, nextWeek.run(ctx))
	assert.Equal(t, 3, countNotifications(t, ctx, pool, "weekly_rotation"))
	var groupID string
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT group_id FROM notification WHERE group_id LIKE 'weekly_rotation:2026-38:%'`).Scan(&groupID))
	assert.Equal(t, "weekly_rotation:2026-38:1", groupID)
}

func TestWeeklyRotationNotifications_SendWindow(t *testing.T) {
	pool := database.CreateTestDatabase(t, "test_jobs")
	defer pool.Close()
	ctx := context.Background()

	seedWeeklyRotationListeners(pool, weeklyRotationSendInstant)

	cases := []struct {
		name string
		now  time.Time
		sent bool
	}{
		{"rollover instant, before send hour", weeklyRotationSendInstant.Add(-16 * time.Hour), false},
		{"one second before send hour", weeklyRotationSendInstant.Add(-time.Second), false},
		{"send hour", weeklyRotationSendInstant, true},
		{"late on rollover day", weeklyRotationSendInstant.Add(7 * time.Hour), true},
		{"one second before the window closes", weeklyRotationSendInstant.Add(24*time.Hour - time.Second), true},
		{"window closed (Thursday 16:00)", weeklyRotationSendInstant.Add(24 * time.Hour), false},
		{"the following Monday", weeklyRotationSendInstant.AddDate(0, 0, 5), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := pool.Exec(ctx, `DELETE FROM notification WHERE type = 'weekly_rotation'`)
			require.NoError(t, err)

			job := newWeeklyRotationJob(pool, tc.now)
			require.NoError(t, job.run(ctx))
			want := 0
			if tc.sent {
				want = 2
			}
			assert.Equal(t, want, countNotifications(t, ctx, pool, "weekly_rotation"))
		})
	}
}

func TestWeeklyRotationNotifications_StageSendsAllWeek(t *testing.T) {
	pool := database.CreateTestDatabase(t, "test_jobs")
	defer pool.Close()
	ctx := context.Background()

	// Rollover instant itself, well before the production send hour.
	now := weeklyRotationSendInstant.Add(-16 * time.Hour)
	seedWeeklyRotationListeners(pool, now)

	job := newWeeklyRotationJob(pool, now)
	job.env = "stage"
	require.NoError(t, job.run(ctx))
	assert.Equal(t, 2, countNotifications(t, ctx, pool, "weekly_rotation"))
}

func TestWeeklyRotationNotifications_Pacing(t *testing.T) {
	pool := database.CreateTestDatabase(t, "test_jobs")
	defer pool.Close()
	ctx := context.Background()

	now := weeklyRotationSendInstant
	seedWeeklyRotationListeners(pool, now)

	job := newWeeklyRotationJob(pool, now)
	job.batchSize = 1

	require.NoError(t, job.run(ctx))
	assert.Equal(t, 1, countNotifications(t, ctx, pool, "weekly_rotation"))
	assert.Equal(t, int32(1), job.cursorUserId)

	require.NoError(t, job.run(ctx))
	assert.Equal(t, 2, countNotifications(t, ctx, pool, "weekly_rotation"))
	assert.Equal(t, int32(4), job.cursorUserId)

	// Drained: further runs are no-ops.
	require.NoError(t, job.run(ctx))
	assert.Equal(t, 2, countNotifications(t, ctx, pool, "weekly_rotation"))
}
