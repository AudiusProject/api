package challenges

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// ProfileVerifiedProcessor implements challenge "v" — boolean: user is
// verified (connected Twitter/Instagram or similar). Mirrors apps'
// connect_verified_challenge.py.
//
// We complete the challenge for any user with users.is_verified = true,
// is_current = true, is_deactivated = false. Boolean — single step.
type ProfileVerifiedProcessor struct{}

func (p *ProfileVerifiedProcessor) ChallengeID() string { return "v" }

func (p *ProfileVerifiedProcessor) Reconcile(ctx context.Context, tx pgx.Tx) error {
	c, ok, err := LoadChallenge(ctx, tx, p.ChallengeID())
	if err != nil {
		return fmt.Errorf("load challenge: %w", err)
	}
	if !ok || !c.Active {
		return nil
	}
	amount := c.AmountInt()

	rows, err := tx.Query(ctx, `
		SELECT user_id FROM users
		WHERE is_current = true
		  AND is_verified = true
		  AND is_deactivated = false
	`)
	if err != nil {
		return fmt.Errorf("scan verified users: %w", err)
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
