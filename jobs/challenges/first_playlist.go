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
type FirstPlaylistProcessor struct{}

func (p *FirstPlaylistProcessor) ChallengeID() string { return "fp" }

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

	// Find every user with at least one non-deleted playlist at or after
	// the starting block. Boolean challenges complete in a single step
	// (step_count is null/0 — we treat current_step_count=1, step=1).
	rows, err := tx.Query(ctx, `
		SELECT DISTINCT playlist_owner_id
		FROM playlists
		WHERE is_current = true
		  AND is_delete = false
		  AND blocknumber >= $1
	`, startingBlock)
	if err != nil {
		return fmt.Errorf("scan playlists: %w", err)
	}
	var userIDs []int64
	for rows.Next() {
		var userID int64
		if err := rows.Scan(&userID); err != nil {
			rows.Close()
			return err
		}
		userIDs = append(userIDs, userID)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}

	for _, userID := range userIDs {
		if err := UpsertUserChallenge(ctx, tx,
			p.ChallengeID(), SpecifierFromUserID(userID),
			userID, 1, 1, amount,
		); err != nil {
			return fmt.Errorf("upsert: %w", err)
		}
	}
	return nil
}
