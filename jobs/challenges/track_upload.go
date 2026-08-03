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
//
// Incremental: a full GROUP BY over tracks took ~14s against prod every tick.
// We instead recompute only owners whose tracks changed since the checkpoint,
// capping each owner's count at step_count via LIMIT. See incremental.go.
type TrackUploadProcessor struct{}

func (p *TrackUploadProcessor) ChallengeID() string { return "u" }

const trackUploadCheckpoint = "challenges:u:last_blocknumber"

// trackUploadDirtySQL returns (owner_id, blocknumber) for tracks created,
// updated, or deleted since the checkpoint. tracks is updated in place so
// blocknumber moves on every mutation.
const trackUploadDirtySQL = `
	SELECT owner_id, blocknumber FROM tracks
	WHERE blocknumber > $1
	ORDER BY blocknumber ASC
	LIMIT $2
`

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

	return reconcileIncrementalUsers(ctx, tx, trackUploadCheckpoint, trackUploadDirtySQL,
		func(ctx context.Context, tx pgx.Tx, ownerIDs []int64) error {
			return p.recompute(ctx, tx, ownerIDs, stepCount, startingBlock, amount)
		})
}

func (p *TrackUploadProcessor) recompute(ctx context.Context, tx pgx.Tx, ownerIDs []int64, stepCount, startingBlock, amount int32) error {
	// Per-owner qualifying-track count, capped at step_count. The cap (LIMIT)
	// keeps a label with thousands of tracks from being a full count.
	rows, err := tx.Query(ctx, `
		SELECT x.owner_id, fc.cnt
		FROM unnest($1::bigint[]) AS x(owner_id)
		JOIN LATERAL (
			SELECT count(*)::int AS cnt
			FROM (
				SELECT 1 FROM tracks t
				WHERE t.owner_id = x.owner_id
				  AND t.is_current = true
				  AND t.is_delete = false
				  AND t.is_unlisted = false
				  AND t.stem_of IS NULL
				  AND t.blocknumber >= $2
				LIMIT $3
			) z
		) fc ON true
	`, ownerIDs, startingBlock, stepCount)
	if err != nil {
		return fmt.Errorf("count tracks: %w", err)
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
		// Skip owners with no qualifying tracks (e.g. all deleted) — matches
		// the old GROUP BY, which only emitted rows for owners with tracks,
		// and never downgrades an already-complete row (sticky is_complete).
		if r.count == 0 {
			continue
		}
		if err := UpsertUserChallenge(ctx, tx,
			p.ChallengeID(), SpecifierFromUserID(r.userID),
			r.userID, r.count, stepCount, amount,
		); err != nil {
			return fmt.Errorf("upsert user_challenge: %w", err)
		}
	}
	return nil
}
