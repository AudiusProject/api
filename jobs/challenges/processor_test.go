package challenges

import (
	"context"
	"testing"

	"api.audius.co/database"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

// withChallengesDB returns a test DB pool with the Phase 1 challenges
// catalog rows seeded. Each test that calls this re-seeds from scratch.
func withChallengesDB(t *testing.T) *pgxpool.Pool {
	t.Helper()
	pool := database.CreateTestDatabase(t, "test_jobs")
	t.Cleanup(func() { pool.Close() })

	ctx := context.Background()

	// Seed Phase 1+2+3 challenges catalog inline. We don't run the
	// production migration here because the test_jobs template DB isn't
	// routed through the ddl runner — keeping the seed local to the
	// test makes intent clearer too.
	rows := []struct {
		id, typ, amount string
		active          bool
		stepCount       *int32
		startingBlock   int32
		weeklyPool      int32
		cooldownDays    *int32
	}{
		{"p", "numeric", "1", true, i32p(7), 0, 25000, i32p(7)},
		{"u", "numeric", "1", true, i32p(3), 25346436, 25000, i32p(7)},
		{"fp", "boolean", "2", true, nil, 28350000, 25000, i32p(7)},
		{"v", "boolean", "5", true, nil, 0, 25000, nil},
		{"e", "aggregate", "1", false, i32p(2147483647), 116023891, 25000, nil},
		{"p1", "numeric", "25", true, i32p(250), 0, 2147483647, i32p(7)},
		{"p2", "numeric", "100", true, i32p(1000), 0, 2147483647, i32p(7)},
		{"p3", "numeric", "1000", true, i32p(10000), 0, 2147483647, i32p(7)},
		{"tt", "trending", "1000", true, nil, 25346436, 100000, nil},
		{"tut", "trending", "1000", true, nil, 25346436, 100000, nil},
		{"tp", "trending", "100", true, nil, 25346436, 10000, nil},
		// Phase 2
		{"c", "aggregate", "1", true, i32p(2147483647), 0, 2147483647, i32p(7)},
		{"t", "aggregate", "100", true, i32p(2147483647), 0, 2147483647, i32p(7)},
		{"cp", "aggregate", "10", true, i32p(2147483647), 1979515, 2147483647, i32p(7)},
		{"cs", "aggregate", "1000", false, i32p(2147483647), 95017582, 50000, i32p(7)},
		{"w", "aggregate", "1000", true, i32p(2147483647), 98950182, 50000, i32p(7)},
		{"b", "aggregate", "1", true, i32p(2147483647), 220157041, 25000, i32p(7)},
		{"s", "aggregate", "5", true, i32p(2147483647), 220157041, 25000, i32p(7)},
		// Phase 3 (user_events-sourced)
		{"m", "boolean", "1", true, nil, 25346436, 25000, i32p(7)},
		{"r", "aggregate", "1", true, i32p(5), 25346436, 25000, i32p(7)},
		{"rv", "aggregate", "1", true, i32p(5000), 25346436, 25000, i32p(7)},
		{"rd", "boolean", "1", true, nil, 25346436, 25000, i32p(7)},
	}
	for _, r := range rows {
		_, err := pool.Exec(ctx, `
			INSERT INTO challenges (id, type, amount, active, step_count, starting_block, weekly_pool, cooldown_days)
			VALUES ($1, $2::challengetype, $3, $4, $5, $6, $7, $8)
			ON CONFLICT (id) DO NOTHING
		`, r.id, r.typ, r.amount, r.active, r.stepCount, r.startingBlock, r.weeklyPool, r.cooldownDays)
		require.NoError(t, err)
	}
	return pool
}

func i32p(v int32) *int32 { return &v }

// runProcessor opens a tx, runs Reconcile, and commits — matches the
// umbrella job's per-processor unit-of-work.
func runProcessor(t *testing.T, pool *pgxpool.Pool, p Processor) {
	t.Helper()
	ctx := context.Background()
	tx, err := pool.Begin(ctx)
	require.NoError(t, err)
	defer tx.Rollback(ctx)
	require.NoError(t, p.Reconcile(ctx, tx))
	require.NoError(t, tx.Commit(ctx))
}

// queryUserChallenge returns the user_challenges row for (challenge, specifier).
type ucRow struct {
	UserID           int64
	IsComplete       bool
	CurrentStepCount *int32
	Amount           int32
}

func queryUserChallenge(t *testing.T, pool *pgxpool.Pool, challengeID, specifier string) (ucRow, bool) {
	t.Helper()
	var r ucRow
	err := pool.QueryRow(context.Background(), `
		SELECT user_id, is_complete, current_step_count, amount
		FROM user_challenges
		WHERE challenge_id = $1 AND specifier = $2
	`, challengeID, specifier).Scan(&r.UserID, &r.IsComplete, &r.CurrentStepCount, &r.Amount)
	if err == pgx.ErrNoRows {
		return ucRow{}, false
	}
	require.NoError(t, err)
	return r, true
}
