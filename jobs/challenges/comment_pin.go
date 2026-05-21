package challenges

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// CommentPinProcessor implements challenge "cp" — when a verified track
// owner pins another user's comment on their track, that commenter earns
// the challenge. Mirrors apps' pin_comment dispatch in comment.py:1080.
//
// Specifier (matches apps): <hex_commenter_user_id>:<hex_track_id>
//
// Source data: tracks.pinned_comment_id joined to comments.
// Gates:
//   - track owner must be verified
//   - commenter != track owner (self-pins don't earn)
//
// A pin → unpin → re-pin cycle still only mints one row per (commenter, track)
// thanks to specifier uniqueness, matching apps.
type CommentPinProcessor struct{}

func (p *CommentPinProcessor) ChallengeID() string { return "cp" }

func (p *CommentPinProcessor) Reconcile(ctx context.Context, tx pgx.Tx) error {
	c, ok, err := LoadChallenge(ctx, tx, p.ChallengeID())
	if err != nil {
		return fmt.Errorf("load challenge: %w", err)
	}
	if !ok || !c.Active {
		return nil
	}
	amount := c.AmountInt()

	rows, err := tx.Query(ctx, `
		SELECT cm.user_id AS commenter_user_id, t.track_id
		FROM tracks t
		JOIN comments cm ON cm.comment_id = t.pinned_comment_id
		JOIN users u ON u.user_id = t.owner_id
		WHERE t.pinned_comment_id IS NOT NULL
		  AND t.is_current = true
		  AND t.is_delete = false
		  AND u.is_current = true
		  AND u.is_verified = true
		  AND cm.is_delete = false
		  AND cm.user_id <> t.owner_id
	`)
	if err != nil {
		return fmt.Errorf("scan pinned comments: %w", err)
	}
	type pinRow struct {
		commenterUserID int64
		trackID         int64
	}
	var results []pinRow
	for rows.Next() {
		var r pinRow
		if err := rows.Scan(&r.commenterUserID, &r.trackID); err != nil {
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
		specifier := fmt.Sprintf("%x:%x", r.commenterUserID, r.trackID)
		if err := UpsertUserChallenge(ctx, tx,
			p.ChallengeID(), specifier, r.commenterUserID, 1, 1, amount,
		); err != nil {
			return fmt.Errorf("upsert: %w", err)
		}
	}
	return nil
}
