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

// TestRemixContest_Ended verifies that a remix contest whose end_date fell in
// the last 24h notifies the full fan audience (remixers, host followers, parent
// track favoriters, event subscribers — minus the host) plus the host, and that
// the ending-soon steps stay silent.
func TestRemixContest_Ended(t *testing.T) {
	pool := database.CreateTestDatabase(t, "test_jobs")
	defer pool.Close()

	ctx := context.Background()
	now := time.Date(2025, 1, 15, 12, 0, 0, 0, time.UTC)

	seedRemixContest(t, pool, now.Add(-1*time.Hour))

	job := NewRemixContestNotificationsJob(newTestConfig(), pool)
	job.now = func() time.Time { return now }
	require.NoError(t, job.run(ctx))

	// Fan audience: remixer(2), follower(3), favoriter(4), subscriber(5).
	assert.Equal(t, 4, countNotifications(t, ctx, pool, "fan_remix_contest_ended"))
	// Host only.
	assert.Equal(t, 1, countNotifications(t, ctx, pool, "artist_remix_contest_ended"))
	// Future-window steps do not fire for an already-ended contest.
	assert.Equal(t, 0, countNotifications(t, ctx, pool, "fan_remix_contest_ending_soon"))
	assert.Equal(t, 0, countNotifications(t, ctx, pool, "artist_remix_contest_ending_soon"))

	// Host is never in the fan audience.
	var fanIDs []int32
	rows, err := pool.Query(ctx,
		`SELECT specifier::int FROM notification WHERE type = 'fan_remix_contest_ended' ORDER BY 1`)
	require.NoError(t, err)
	for rows.Next() {
		var id int32
		require.NoError(t, rows.Scan(&id))
		fanIDs = append(fanIDs, id)
	}
	require.NoError(t, rows.Err())
	assert.Equal(t, []int32{2, 3, 4, 5}, fanIDs)

	// Idempotent.
	require.NoError(t, job.run(ctx))
	assert.Equal(t, 4, countNotifications(t, ctx, pool, "fan_remix_contest_ended"))
	assert.Equal(t, 1, countNotifications(t, ctx, pool, "artist_remix_contest_ended"))
}

// TestRemixContest_EndingSoon verifies that a contest ending within the
// soon-window notifies followers, favoriters, and subscribers (but not
// remixers) plus the host, and that the ended steps stay silent.
func TestRemixContest_EndingSoon(t *testing.T) {
	pool := database.CreateTestDatabase(t, "test_jobs")
	defer pool.Close()

	ctx := context.Background()
	now := time.Date(2025, 1, 15, 12, 0, 0, 0, time.UTC)

	// Within both the 48h artist window and the 72h fan window.
	seedRemixContest(t, pool, now.Add(36*time.Hour))

	job := NewRemixContestNotificationsJob(newTestConfig(), pool)
	job.now = func() time.Time { return now }
	require.NoError(t, job.run(ctx))

	// Fan ending-soon audience excludes remixers: follower(3), favoriter(4), subscriber(5).
	assert.Equal(t, 3, countNotifications(t, ctx, pool, "fan_remix_contest_ending_soon"))
	assert.Equal(t, 1, countNotifications(t, ctx, pool, "artist_remix_contest_ending_soon"))
	assert.Equal(t, 0, countNotifications(t, ctx, pool, "fan_remix_contest_ended"))
	assert.Equal(t, 0, countNotifications(t, ctx, pool, "artist_remix_contest_ended"))

	var fanIDs []int32
	rows, err := pool.Query(ctx,
		`SELECT specifier::int FROM notification WHERE type = 'fan_remix_contest_ending_soon' ORDER BY 1`)
	require.NoError(t, err)
	for rows.Next() {
		var id int32
		require.NoError(t, rows.Scan(&id))
		fanIDs = append(fanIDs, id)
	}
	require.NoError(t, rows.Err())
	assert.Equal(t, []int32{3, 4, 5}, fanIDs)
}

// seedRemixContest sets up a single remix_contest event (event_id 500, host
// user 100, parent track 1000) with one of each fan-audience source and the
// given end_date.
func seedRemixContest(t *testing.T, pool *pgxpool.Pool, endDate time.Time) {
	t.Helper()
	database.Seed(pool, database.FixtureMap{
		"users": {
			{"user_id": 100, "wallet": "0x100"}, // host
			{"user_id": 2, "wallet": "0x02"},    // remixer
			{"user_id": 3, "wallet": "0x03"},    // host follower
			{"user_id": 4, "wallet": "0x04"},    // parent track favoriter
			{"user_id": 5, "wallet": "0x05"},    // event subscriber
		},
		"tracks": {
			{"track_id": 1000, "owner_id": 100, "title": "parent"},
			{"track_id": 1001, "owner_id": 2, "title": "remix",
				"remix_of": `{"tracks":[{"parent_track_id":1000}]}`},
		},
		"events": {
			{"event_id": 500, "entity_type": "track", "user_id": 100, "entity_id": 1000,
				"event_type": "remix_contest", "end_date": endDate},
		},
		"follows": {
			{"follower_user_id": 3, "followee_user_id": 100},
		},
		"saves": {
			{"user_id": 4, "save_item_id": 1000, "save_type": "track"},
		},
		"subscriptions": {
			{"subscriber_id": 5, "user_id": 500, "entity_type": "Event", "entity_id": 1000},
		},
	})
}
