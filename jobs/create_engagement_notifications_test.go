package jobs

import (
	"context"
	"testing"
	"time"

	"api.audius.co/database"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestEngagementNotifications_FullPipeline verifies a completed, undisbursed,
// 7-day-cooldown challenge past its cooldown produces exactly one
// claimable_reward notification, and re-running is idempotent.
func TestEngagementNotifications_FullPipeline(t *testing.T) {
	pool := database.CreateTestDatabase(t, "test_jobs")
	defer pool.Close()

	ctx := context.Background()
	now := time.Date(2025, 1, 15, 12, 0, 0, 0, time.UTC)
	completedAt := now.Add(-8 * 24 * time.Hour) // past the 7-day cooldown

	database.Seed(pool, database.FixtureMap{
		"users": {{"user_id": 1, "wallet": "0x01"}},
		"challenges": {
			{"id": "b", "type": "aggregate", "amount": "10", "active": true, "cooldown_days": 7},
		},
		"user_challenges": {
			{"challenge_id": "b", "user_id": 1, "specifier": "s1", "is_complete": true,
				"amount": 100, "completed_at": completedAt, "completed_blocknumber": 42},
		},
	})

	job := NewEngagementNotificationsJob(newTestConfig(), pool)
	job.now = func() time.Time { return now }

	require.NoError(t, job.run(ctx))

	var typ, groupID, specifier string
	var userIDs []int32
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT type, group_id, specifier, user_ids FROM notification WHERE type = 'claimable_reward'`,
	).Scan(&typ, &groupID, &specifier, &userIDs))
	assert.Equal(t, "claimable_reward", typ)
	assert.Equal(t, "claimable_reward:1:challenge:b:specifier:s1", groupID)
	assert.Equal(t, "s1", specifier)
	assert.Equal(t, []int32{1}, userIDs)

	// Idempotent: a second run inserts nothing more.
	require.NoError(t, job.run(ctx))
	assert.Equal(t, 1, countNotifications(t, ctx, pool, "claimable_reward"))
}

// TestEngagementNotifications_Exclusions verifies the cooldown, disbursement,
// and not-yet-elapsed filters all suppress a claimable_reward from the job.
//
// Note on the test fixtures: the handle_on_user_challenge DB trigger fires when
// a complete user_challenge is seeded. For a challenge with cooldown_days > 0 it
// inserts a `reward_in_cooldown` (never a `claimable_reward`); only a
// cooldown_days = 0 challenge would get an immediate `claimable_reward` from the
// trigger. We therefore use positive, non-7 cooldowns for the "wrong cooldown"
// row so the trigger never produces a claimable_reward, isolating the job's
// behavior — which only ever emits claimable_reward for cooldown_days = 7.
func TestEngagementNotifications_Exclusions(t *testing.T) {
	pool := database.CreateTestDatabase(t, "test_jobs")
	defer pool.Close()

	ctx := context.Background()
	now := time.Date(2025, 1, 15, 12, 0, 0, 0, time.UTC)
	old := now.Add(-8 * 24 * time.Hour)

	database.Seed(pool, database.FixtureMap{
		"users": {{"user_id": 1, "wallet": "0x01"}},
		"challenges": {
			{"id": "cd7", "type": "aggregate", "amount": "10", "active": true, "cooldown_days": 7},
			{"id": "cd14", "type": "aggregate", "amount": "10", "active": true, "cooldown_days": 14},
		},
		"user_challenges": {
			// Disbursed already -> excluded.
			{"challenge_id": "cd7", "user_id": 1, "specifier": "disbursed", "is_complete": true, "amount": 100, "completed_at": old},
			// Wrong cooldown_days (not 7) -> excluded by the job's cooldown_days = 7 filter.
			{"challenge_id": "cd14", "user_id": 1, "specifier": "wrongCooldown", "is_complete": true, "amount": 100, "completed_at": old},
			// Cooldown not yet elapsed -> excluded.
			{"challenge_id": "cd7", "user_id": 1, "specifier": "tooRecent", "is_complete": true, "amount": 100, "completed_at": now.Add(-2 * 24 * time.Hour)},
			// Not complete -> excluded.
			{"challenge_id": "cd7", "user_id": 1, "specifier": "incomplete", "is_complete": false, "amount": 100, "completed_at": old},
		},
		"challenge_disbursements": {
			{"challenge_id": "cd7", "user_id": 1, "specifier": "disbursed", "signature": "sig", "amount": "100"},
		},
	})

	job := NewEngagementNotificationsJob(newTestConfig(), pool)
	job.now = func() time.Time { return now }
	require.NoError(t, job.run(ctx))

	assert.Equal(t, 0, countNotifications(t, ctx, pool, "claimable_reward"))
}

func countNotifications(t *testing.T, ctx context.Context, pool database.DbPool, typ string) int {
	t.Helper()
	var n int
	require.NoError(t, pool.QueryRow(ctx,
		"SELECT COUNT(*) FROM notification WHERE type = $1", typ).Scan(&n))
	return n
}
