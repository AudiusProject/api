package jobs

import (
	"context"
	"testing"
	"time"

	"api.audius.co/database"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestListenStreakReminder_Window verifies that only users whose last listen
// falls in the 42-43h reminder window are notified, and that re-running is
// idempotent for the same (user, last-listen date).
func TestListenStreakReminder_Window(t *testing.T) {
	pool := database.CreateTestDatabase(t, "test_jobs")
	defer pool.Close()

	ctx := context.Background()
	now := time.Date(2025, 1, 15, 12, 0, 0, 0, time.UTC)

	database.Seed(pool, database.FixtureMap{
		"users": {
			{"user_id": 1, "wallet": "0x01"},
			{"user_id": 2, "wallet": "0x02"},
			{"user_id": 3, "wallet": "0x03"},
		},
		"challenge_listen_streak": {
			// In window (42.5h ago) -> notified.
			{"user_id": 1, "listen_streak": 5, "last_listen_date": now.Add(-42*time.Hour - 30*time.Minute)},
			// Too recent (40h ago) -> excluded.
			{"user_id": 2, "listen_streak": 3, "last_listen_date": now.Add(-40 * time.Hour)},
			// Too old (50h ago, streak already broken) -> excluded.
			{"user_id": 3, "listen_streak": 9, "last_listen_date": now.Add(-50 * time.Hour)},
		},
	})

	job := NewListenStreakReminderJob(newTestConfig(), pool)
	job.now = func() time.Time { return now }
	require.NoError(t, job.run(ctx))

	var typ, groupID, specifier string
	var userIDs []int32
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT type, group_id, specifier, user_ids FROM notification WHERE type = 'listen_streak_reminder'`,
	).Scan(&typ, &groupID, &specifier, &userIDs))
	assert.Equal(t, "listen_streak_reminder", typ)
	assert.Equal(t, "listen_streak_reminder:1:2025-01-13", groupID)
	assert.Equal(t, "1", specifier)
	assert.Equal(t, []int32{1}, userIDs)

	assert.Equal(t, 1, countNotifications(t, ctx, pool, "listen_streak_reminder"))

	// Idempotent: a second run inserts nothing more.
	require.NoError(t, job.run(ctx))
	assert.Equal(t, 1, countNotifications(t, ctx, pool, "listen_streak_reminder"))
}
