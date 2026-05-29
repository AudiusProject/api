package challenges

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// TrackUploadProcessor implements challenge "u" — upload 3 tracks.
// Mirrors apps/packages/discovery-provider/src/challenges/track_upload_challenge.py.
//
// One row per user in user_challenges with specifier = hex(user_id).
// Step count is the number of public, non-stem tracks the user owns since
// challenges.starting_block. Completes when count >= challenge.step_count
// (3 per challenges.json).
type TrackUploadProcessor struct{}

func (p *TrackUploadProcessor) ChallengeID() string { return "u" }

func (p *TrackUploadProcessor) Reconcile(ctx context.Context, tx pgx.Tx) error {
	c, ok, err := LoadChallenge(ctx, tx, p.ChallengeID())
	if err != nil {
		return fmt.Errorf("load challenge: %w", err)
	}
	if !ok || !c.Active || c.StepCount == nil || c.StartingBlock == nil {
		return nil
	}
	stepCount := *c.StepCount
	startingBlock := *c.StartingBlock
	amount := c.AmountInt()

	// For every user with at least one qualifying track at or after the
	// starting block, count tracks and write user_challenges. We don't
	// checkpoint by user id because Python's behavior is to recompute
	// from source every time the challenge processes — keeping that
	// avoids edge cases where a track gets undeleted post-checkpoint.
	rows, err := tx.Query(ctx, `
		SELECT owner_id, COUNT(*)::int
		FROM tracks
		WHERE is_current = true
		  AND is_delete = false
		  AND is_unlisted = false
		  AND stem_of IS NULL
		  AND blocknumber >= $1
		GROUP BY owner_id
	`, startingBlock)
	if err != nil {
		return fmt.Errorf("scan tracks: %w", err)
	}
	type res struct {
		userID int64
		count  int32
	}
	var results []res
	for rows.Next() {
		var r res
		if err := rows.Scan(&r.userID, &r.count); err != nil {
			rows.Close()
			return err
		}
		results = append(results, r)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}

	for _, r := range results {
		// Cap reported step count at the step_count target so we don't
		// confuse readers about "is_complete" semantics.
		report := r.count
		if stepCount > 0 && report > stepCount {
			report = stepCount
		}
		if err := UpsertUserChallenge(ctx, tx,
			p.ChallengeID(), SpecifierFromUserID(r.userID),
			r.userID, report, stepCount, amount,
		); err != nil {
			return fmt.Errorf("upsert user_challenge: %w", err)
		}
	}
	return nil
}
