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
// send instant for the 2026-37 recap (2026-09-09 to 2026-09-16).
var weeklyArtistRecapSendInstant = time.Date(2026, time.September, 16, 18, 0, 0, 0, time.UTC)

func seedWeeklyArtistRecap(t *testing.T, pool *pgxpool.Pool) {
	inWeek := weeklyrotation.PeriodStart(2026, 37).Add(36 * time.Hour)
	beforeWeek := weeklyrotation.PeriodStart(2026, 37).Add(-time.Minute)
	nextWeek := weeklyrotation.PeriodStart(2026, 38).Add(time.Hour)

	play := func(id, userId, trackId int, city any, at time.Time) map[string]any {
		var u any = userId
		if userId == 0 {
			u = nil
		}
		return map[string]any{"id": id, "user_id": u, "play_item_id": trackId, "city": city, "created_at": at}
	}

	database.Seed(pool, database.FixtureMap{
		"users": {
			{"user_id": 1, "handle": "listener1", "wallet": "0x01"},
			{"user_id": 2, "handle": "listener2", "wallet": "0x02"},
			{"user_id": 3, "handle": "listener3", "wallet": "0x03"},
			{"user_id": 10, "handle": "busy", "wallet": "0x10"},
			{"user_id": 20, "handle": "quiet", "wallet": "0x20"},
			{"user_id": 30, "handle": "deactivated", "wallet": "0x30", "is_deactivated": true},
			{"user_id": 40, "handle": "self_only", "wallet": "0x40"},
			{"user_id": 50, "handle": "no_plays", "wallet": "0x50"},
			{"user_id": 60, "handle": "next_week", "wallet": "0x60"},
		},
		"tracks": {
			{"track_id": 101, "owner_id": 10, "title": "a"},
			{"track_id": 102, "owner_id": 10, "title": "b"},
			{"track_id": 201, "owner_id": 20, "title": "c"},
			{"track_id": 301, "owner_id": 30, "title": "d"},
			{"track_id": 401, "owner_id": 40, "title": "e"},
			{"track_id": 501, "owner_id": 50, "title": "f"},
			{"track_id": 601, "owner_id": 60, "title": "g"},
		},
		"plays": {
			// Artist 10: 4 plays this week, one anonymous.
			play(1, 1, 101, "Atlanta", inWeek),
			play(2, 2, 101, "Atlanta", inWeek),
			play(3, 3, 102, "Berlin", inWeek),
			play(4, 0, 102, nil, inWeek),
			// Last week and next week don't count.
			play(5, 1, 101, "Paris", beforeWeek),
			play(6, 1, 101, "Paris", nextWeek),
			// Artist 20: one play.
			play(7, 1, 201, "", inWeek),
			// Artist 30 is deactivated; 40 only played their own track.
			play(8, 1, 301, "Atlanta", inWeek),
			play(9, 40, 401, "Atlanta", inWeek),
			// Artist 60 only has plays in the in-progress period.
			play(10, 1, 601, "Lagos", nextWeek),
		},
		"follows": {
			{"follower_user_id": 1, "followee_user_id": 10, "created_at": inWeek},
			{"follower_user_id": 2, "followee_user_id": 10, "created_at": inWeek},
			// Unfollowed, and followed before the week: neither counts.
			{"follower_user_id": 3, "followee_user_id": 10, "created_at": inWeek, "is_delete": true},
			{"follower_user_id": 40, "followee_user_id": 10, "created_at": beforeWeek},
			{"follower_user_id": 1, "followee_user_id": 50, "created_at": inWeek},
		},
		"saves": {
			{"user_id": 1, "save_item_id": 101, "save_type": "track", "created_at": inWeek},
			{"user_id": 2, "save_item_id": 102, "save_type": "track", "created_at": inWeek},
			{"user_id": 3, "save_item_id": 101, "save_type": "track", "created_at": inWeek},
			{"user_id": 3, "save_item_id": 102, "save_type": "track", "created_at": nextWeek},
			{"user_id": 1, "save_item_id": 501, "save_type": "track", "created_at": inWeek},
		},
	})

	// Listener 1 had 101 in their 2026-37 rotation, listener 2 had 102 but
	// past the shown slots, and listener 3 had 101 only in the next period.
	seedWeeklyRotationMix(t, pool, 1, 2026, 37, []int32{101})
	listener2 := []int32{}
	for len(listener2) < weeklyrotation.ShownSize {
		listener2 = append(listener2, 201)
	}
	seedWeeklyRotationMix(t, pool, 2, 2026, 37, append(listener2, 102))
	seedWeeklyRotationMix(t, pool, 3, 2026, 38, []int32{101})
}

func newWeeklyArtistRecapJob(pool database.DbPool, now time.Time) *WeeklyArtistRecapNotificationsJob {
	job := NewWeeklyArtistRecapNotificationsJob(newTestConfig(), pool)
	job.now = func() time.Time { return now }
	return job
}

func TestWeeklyArtistRecapNotifications_FanOut(t *testing.T) {
	pool := database.CreateTestDatabase(t, "test_jobs")
	defer pool.Close()
	ctx := context.Background()
	seedWeeklyArtistRecap(t, pool)

	now := weeklyArtistRecapSendInstant.Add(30 * time.Minute)
	job := newWeeklyArtistRecapJob(pool, now)
	require.NoError(t, job.run(ctx))

	rows, err := pool.Query(ctx, `
		SELECT specifier, group_id, user_ids, data, timestamp
		FROM notification WHERE type = 'weekly_artist_recap' ORDER BY specifier::int`)
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
	// 30 (deactivated), 40 (self plays only), 50 (no plays) and 60 (plays
	// only in the in-progress period) get nothing.
	require.Len(t, got, 2)

	assert.Equal(t, "10", got[0].specifier)
	assert.Equal(t, "weekly_artist_recap:2026-37:10", got[0].groupID)
	assert.Equal(t, []int32{10}, got[0].userIDs)
	assert.Equal(t, map[string]any{
		"year": float64(2026), "week": float64(37),
		"play_count":          float64(4),
		"follower_count":      float64(2),
		"save_count":          float64(3),
		"rotation_save_count": float64(1),
	}, got[0].data)
	assert.WithinDuration(t, now, got[0].ts, time.Second)

	assert.Equal(t, "20", got[1].specifier)
	assert.Equal(t, map[string]any{
		"year": float64(2026), "week": float64(37),
		"play_count":          float64(1),
		"follower_count":      float64(0),
		"save_count":          float64(0),
		"rotation_save_count": float64(0),
	}, got[1].data)

	// Idempotent: the same job is drained, and a restart adds nothing.
	assert.True(t, job.drained)
	require.NoError(t, job.run(ctx))
	fresh := newWeeklyArtistRecapJob(pool, now)
	require.NoError(t, fresh.run(ctx))
	assert.Equal(t, 2, countNotifications(t, ctx, pool, "weekly_artist_recap"))

	// The next rollover reports 2026-38.
	nextWeek := newWeeklyArtistRecapJob(pool, now.AddDate(0, 0, 7))
	require.NoError(t, nextWeek.run(ctx))
	var groupIDs []string
	rows2, err := pool.Query(ctx,
		`SELECT group_id FROM notification WHERE group_id LIKE 'weekly_artist_recap:2026-38:%' ORDER BY group_id`)
	require.NoError(t, err)
	for rows2.Next() {
		var g string
		require.NoError(t, rows2.Scan(&g))
		groupIDs = append(groupIDs, g)
	}
	assert.Equal(t, []string{"weekly_artist_recap:2026-38:10", "weekly_artist_recap:2026-38:60"}, groupIDs)
}

func TestWeeklyArtistRecapNotifications_SendWindow(t *testing.T) {
	pool := database.CreateTestDatabase(t, "test_jobs")
	defer pool.Close()
	ctx := context.Background()
	seedWeeklyArtistRecap(t, pool)

	cases := []struct {
		name string
		now  time.Time
		sent bool
	}{
		{"rollover instant, before send hour", weeklyArtistRecapSendInstant.Add(-18 * time.Hour), false},
		{"one second before send hour", weeklyArtistRecapSendInstant.Add(-time.Second), false},
		{"send hour", weeklyArtistRecapSendInstant, true},
		{"one second before the window closes", weeklyArtistRecapSendInstant.Add(24*time.Hour - time.Second), true},
		{"window closed (Thursday 18:00)", weeklyArtistRecapSendInstant.Add(24 * time.Hour), false},
		{"the following Monday", weeklyArtistRecapSendInstant.AddDate(0, 0, 5), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := pool.Exec(ctx, `DELETE FROM notification WHERE type = 'weekly_artist_recap'`)
			require.NoError(t, err)

			job := newWeeklyArtistRecapJob(pool, tc.now)
			require.NoError(t, job.run(ctx))
			want := 0
			if tc.sent {
				want = 2
			}
			assert.Equal(t, want, countNotifications(t, ctx, pool, "weekly_artist_recap"))
		})
	}
}

func TestWeeklyArtistRecapNotifications_StageSendsAllWeek(t *testing.T) {
	pool := database.CreateTestDatabase(t, "test_jobs")
	defer pool.Close()
	ctx := context.Background()
	seedWeeklyArtistRecap(t, pool)

	job := newWeeklyArtistRecapJob(pool, weeklyArtistRecapSendInstant.Add(-18*time.Hour))
	job.env = "stage"
	require.NoError(t, job.run(ctx))
	assert.Equal(t, 2, countNotifications(t, ctx, pool, "weekly_artist_recap"))
}

func TestWeeklyArtistRecapNotifications_Pacing(t *testing.T) {
	pool := database.CreateTestDatabase(t, "test_jobs")
	defer pool.Close()
	ctx := context.Background()
	seedWeeklyArtistRecap(t, pool)

	job := newWeeklyArtistRecapJob(pool, weeklyArtistRecapSendInstant)
	job.batchSize = 1

	require.NoError(t, job.run(ctx))
	assert.Equal(t, 1, countNotifications(t, ctx, pool, "weekly_artist_recap"))
	assert.Equal(t, int32(10), job.cursorOwnerId)
	assert.False(t, job.drained)

	require.NoError(t, job.run(ctx))
	assert.Equal(t, 2, countNotifications(t, ctx, pool, "weekly_artist_recap"))
	assert.Equal(t, int32(20), job.cursorOwnerId)

	// The next run finds nothing past the cursor and marks the period drained.
	require.NoError(t, job.run(ctx))
	assert.Equal(t, 2, countNotifications(t, ctx, pool, "weekly_artist_recap"))
	assert.True(t, job.drained)
}
