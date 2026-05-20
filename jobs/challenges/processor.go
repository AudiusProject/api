// Package challenges implements the discovery-provider "challenges" reward
// system in api/. Each challenge is a Processor that scans source tables on
// a schedule and reconciles state in user_challenges + the per-challenge
// state table.
//
// Architecture vs the legacy Python stack:
//
//   - Python uses a Redis event bus: every entity_manager handler dispatches
//     a ChallengeEvent (track_upload, follow, favorite, etc.), index_challenges
//     drains the queue and calls per-challenge ChallengeManager.process().
//   - We don't have a queue. Producers (chain indexer) write base rows;
//     consumers (these processors) reconcile derived state from those rows on
//     a tick. This is idempotent, restart-safe, and backfill-friendly. The
//     consistency model matches Python's anyway: Python's challenge processing
//     is async and rate-limited; we're just batching the work differently.
//
// Each processor owns:
//
//   - A ChallengeID() returning the catalog id from the `challenges` table.
//   - A Reconcile(ctx, tx) that reads source tables and applies updates to
//     user_challenges (+ any per-challenge state table like
//     challenge_profile_completion).
//
// The umbrella IndexChallengesJob runs them sequentially per tick. Failures
// in one processor don't stop others — each runs in its own transaction.
package challenges

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// Processor is the per-challenge interface. Implementations should be
// stateless w.r.t. process memory (state lives in DB).
type Processor interface {
	// ChallengeID is the row id in the `challenges` table.
	ChallengeID() string
	// Reconcile reads source tables and applies updates. Must be
	// idempotent — re-running with no source changes must not change state.
	// Tx is begun + committed by the umbrella job; processors should not
	// commit or rollback themselves.
	Reconcile(ctx context.Context, tx pgx.Tx) error
}

// LoadChallenge reads a single row from `challenges` by id.
// Returns ok=false if the row is absent.
func LoadChallenge(ctx context.Context, tx pgx.Tx, id string) (Challenge, bool, error) {
	var c Challenge
	err := tx.QueryRow(ctx, `
		SELECT id, type::text, amount, active, step_count, starting_block, weekly_pool, cooldown_days
		FROM challenges WHERE id = $1
	`, id).Scan(&c.ID, &c.Type, &c.Amount, &c.Active, &c.StepCount, &c.StartingBlock, &c.WeeklyPool, &c.CooldownDays)
	if errors.Is(err, pgx.ErrNoRows) {
		return Challenge{}, false, nil
	}
	return c, err == nil, err
}

// Challenge mirrors the public.challenges row shape.
type Challenge struct {
	ID            string
	Type          string
	Amount        string // varchar in DB
	Active        bool
	StepCount     *int32
	StartingBlock *int32
	WeeklyPool    *int32
	CooldownDays  *int32
}

// AmountInt parses Amount, defaulting to 0 on error.
func (c Challenge) AmountInt() int32 {
	v := int32(0)
	if _, err := fmt.Sscanf(c.Amount, "%d", &v); err != nil {
		return 0
	}
	return v
}

// SpecifierFromUserID matches Python's default ChallengeUpdater.generate_specifier:
//
//	hex(user_id)[2:]
//
// Lowercase hex, no "0x" prefix, no left-padding.
func SpecifierFromUserID(userID int64) string {
	return fmt.Sprintf("%x", userID)
}

// UpsertUserChallenge inserts a user_challenges row or updates progress.
// Mirrors the rows ChallengeManager.process() would write in Python.
//
// completedAt is set/cleared from current_step_count >= stepCount; when a
// row newly completes we use blockTime for completed_at if non-zero, else now().
func UpsertUserChallenge(
	ctx context.Context,
	tx pgx.Tx,
	challengeID, specifier string,
	userID int64,
	currentStepCount, stepCount int32,
	amount int32,
) error {
	isComplete := stepCount > 0 && currentStepCount >= stepCount
	_, err := tx.Exec(ctx, `
		INSERT INTO user_challenges
			(challenge_id, user_id, specifier, is_complete, current_step_count, amount, created_at, completed_at)
		VALUES ($1, $2, $3, $4, $5, $6, now(), CASE WHEN $4 THEN now() ELSE NULL END)
		ON CONFLICT (challenge_id, specifier) DO UPDATE SET
			current_step_count = EXCLUDED.current_step_count,
			is_complete = user_challenges.is_complete OR EXCLUDED.is_complete,
			amount = CASE
				WHEN user_challenges.is_complete THEN user_challenges.amount
				ELSE EXCLUDED.amount
			END,
			completed_at = CASE
				WHEN user_challenges.is_complete THEN user_challenges.completed_at
				WHEN EXCLUDED.is_complete THEN now()
				ELSE user_challenges.completed_at
			END
	`, challengeID, userID, specifier, isComplete, currentStepCount, amount)
	return err
}
