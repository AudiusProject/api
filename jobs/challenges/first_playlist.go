package challenges

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// FirstPlaylistProcessor implements challenge "fp" — boolean: the user has
// created at least one playlist.
// Mirrors apps/packages/discovery-provider/src/challenges/first_playlist_challenge.py
// (Python just sets is_complete=true when an event fires; we derive it from
// playlists table state directly).
//
// Incremental: the old version re-scanned all 311K playlists and re-upserted
// every distinct owner on every tick. We instead recompute only owners whose
// playlists changed since the checkpoint. See incremental.go.
type FirstPlaylistProcessor struct{}

func (p *FirstPlaylistProcessor) ChallengeID() string { return "fp" }

const firstPlaylistCheckpoint = "challenges:fp:last_blocknumber"

// firstPlaylistDirtySQL returns (playlist_owner_id, blocknumber) for playlists
// changed since the checkpoint. playlists is updated in place so blocknumber
// moves on every create/update/delete.
const firstPlaylistDirtySQL = `
	SELECT playlist_owner_id, blocknumber FROM playlists
	WHERE blocknumber > $1
	ORDER BY blocknumber ASC
	LIMIT $2
`

func (p *FirstPlaylistProcessor) Reconcile(ctx context.Context, tx pgx.Tx) error {
	c, ok, err := LoadChallenge(ctx, tx, p.ChallengeID())
	if err != nil {
		return fmt.Errorf("load challenge: %w", err)
	}
	if !ok || !c.Active {
		return nil
	}
	startingBlock := int32(0)
	if c.StartingBlock != nil {
		startingBlock = *c.StartingBlock
	}
	amount := c.AmountInt()

	return reconcileIncrementalUsers(ctx, tx, firstPlaylistCheckpoint, firstPlaylistDirtySQL,
		func(ctx context.Context, tx pgx.Tx, ownerIDs []int64) error {
			return p.recompute(ctx, tx, ownerIDs, startingBlock, amount)
		})
}

func (p *FirstPlaylistProcessor) recompute(ctx context.Context, tx pgx.Tx, ownerIDs []int64, startingBlock, amount int32) error {
	// Keep only owners that currently have a non-deleted playlist at/after the
	// starting block. Boolean challenge: complete in a single step.
	rows, err := tx.Query(ctx, `
		SELECT x.owner_id
		FROM unnest($1::bigint[]) AS x(owner_id)
		WHERE EXISTS (
			SELECT 1 FROM playlists pl
			WHERE pl.playlist_owner_id = x.owner_id
			  AND pl.is_current = true
			  AND pl.is_delete = false
			  AND pl.blocknumber >= $2
		)
	`, ownerIDs, startingBlock)
	if err != nil {
		return fmt.Errorf("scan playlists: %w", err)
	}
	var qualifying []int64
	for rows.Next() {
		var userID int64
		if err := rows.Scan(&userID); err != nil {
			rows.Close()
			return err
		}
		qualifying = append(qualifying, userID)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}

	for _, userID := range qualifying {
		if err := UpsertUserChallenge(ctx, tx,
			p.ChallengeID(), SpecifierFromUserID(userID),
			userID, 1, 1, amount,
		); err != nil {
			return fmt.Errorf("upsert: %w", err)
		}
	}
	return nil
}
