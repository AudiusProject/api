// Package challenges, signal-driven processors.
//
// Phase 3 processors (m, r, rv, rd) consume from the challenge_signals
// table (populated by POST /v1/challenges/signals or by admins via SQL).
// Each processor reads its slice of signals since checkpoint and mints
// the appropriate user_challenges row.
package challenges

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// signalsCheckpointName returns the indexing_checkpoints key for a
// given (challenge_id, signal_type) tuple. Each processor advances its
// own cursor independently — multiple processors can read the same
// signal_type without interfering.
func signalsCheckpointName(challengeID, signalType string) string {
	return "challenges:" + challengeID + ":signals:" + signalType
}

// signalRow is a row from challenge_signals.
type signalRow struct {
	ID        int64
	UserID    int64
	ExtraJSON []byte
	Source    *string
}

// readSignalsSince fetches up to `limit` rows of the given type with
// id > prev, ordered by id.
func readSignalsSince(ctx context.Context, tx pgx.Tx, signalType string, prev int64, limit int) ([]signalRow, int64, error) {
	rows, err := tx.Query(ctx, `
		SELECT id, user_id, extra, source
		FROM challenge_signals
		WHERE type = $1::challenge_signal_type AND id > $2
		ORDER BY id ASC
		LIMIT $3
	`, signalType, prev, limit)
	if err != nil {
		return nil, prev, err
	}
	defer rows.Close()
	var out []signalRow
	maxID := prev
	for rows.Next() {
		var r signalRow
		if err := rows.Scan(&r.ID, &r.UserID, &r.ExtraJSON, &r.Source); err != nil {
			return nil, prev, err
		}
		out = append(out, r)
		if r.ID > maxID {
			maxID = r.ID
		}
	}
	return out, maxID, rows.Err()
}

func decodeSignalExtra(blob []byte, dest any) error {
	if len(blob) == 0 {
		return nil
	}
	return json.Unmarshal(blob, dest)
}

// MobileInstallProcessor implements challenge "m" — boolean reward when
// a user reports installing the mobile app. Single signal type:
// "mobile_install"; signal user_id == reward target.
type MobileInstallProcessor struct{}

func (p *MobileInstallProcessor) ChallengeID() string { return "m" }

func (p *MobileInstallProcessor) Reconcile(ctx context.Context, tx pgx.Tx) error {
	c, ok, err := LoadChallenge(ctx, tx, p.ChallengeID())
	if err != nil {
		return fmt.Errorf("load challenge: %w", err)
	}
	if !ok || !c.Active {
		return nil
	}
	amount := c.AmountInt()
	cpName := signalsCheckpointName(p.ChallengeID(), "mobile_install")
	prev, err := readCheckpointInt(ctx, tx, cpName)
	if err != nil {
		return err
	}
	signals, maxID, err := readSignalsSince(ctx, tx, "mobile_install", prev, 5000)
	if err != nil {
		return fmt.Errorf("read signals: %w", err)
	}
	for _, s := range signals {
		if err := UpsertUserChallenge(ctx, tx,
			p.ChallengeID(), SpecifierFromUserID(s.UserID),
			s.UserID, 1, 1, amount,
		); err != nil {
			return fmt.Errorf("upsert: %w", err)
		}
	}
	if maxID > prev {
		if err := writeCheckpointInt(ctx, tx, cpName, maxID); err != nil {
			return err
		}
	}
	return nil
}

// ReferralProcessor implements challenges "r" / "rv" — referrer earns.
// Signal type: "referral". The signal's user_id is the *referred* user
// (who reported their referrer); extra.referrer_user_id is the existing
// user who gets the reward. Gates differ by challenge:
//
//	r:  referrer NOT verified
//	rv: referrer IS verified
type ReferralProcessor struct {
	ID       string // "r" or "rv"
	Verified bool   // gate: referrer must be verified (rv) or not (r)
}

func NewReferralProcessor() Processor          { return &ReferralProcessor{ID: "r", Verified: false} }
func NewVerifiedReferralProcessor() Processor  { return &ReferralProcessor{ID: "rv", Verified: true} }

func (p *ReferralProcessor) ChallengeID() string { return p.ID }

func (p *ReferralProcessor) Reconcile(ctx context.Context, tx pgx.Tx) error {
	c, ok, err := LoadChallenge(ctx, tx, p.ChallengeID())
	if err != nil {
		return fmt.Errorf("load challenge: %w", err)
	}
	if !ok || !c.Active {
		return nil
	}
	amount := c.AmountInt()
	cpName := signalsCheckpointName(p.ChallengeID(), "referral")
	prev, err := readCheckpointInt(ctx, tx, cpName)
	if err != nil {
		return err
	}
	signals, maxID, err := readSignalsSince(ctx, tx, "referral", prev, 5000)
	if err != nil {
		return err
	}
	for _, s := range signals {
		var extra struct {
			ReferrerUserID *int64 `json:"referrer_user_id"`
		}
		if err := decodeSignalExtra(s.ExtraJSON, &extra); err != nil {
			return fmt.Errorf("decode referral extra: %w", err)
		}
		if extra.ReferrerUserID == nil {
			continue // malformed signal — skip
		}
		// Look up referrer's verification status.
		var verified bool
		if err := tx.QueryRow(ctx, `
			SELECT COALESCE(is_verified, false) FROM users
			WHERE user_id = $1 AND is_current = true
			LIMIT 1
		`, *extra.ReferrerUserID).Scan(&verified); err != nil {
			if err == pgx.ErrNoRows {
				continue // referrer doesn't exist — skip
			}
			return err
		}
		if verified != p.Verified {
			continue // wrong gate for this processor
		}
		// Specifier: <hex_referrer>:<hex_referred>
		specifier := fmt.Sprintf("%x:%x", *extra.ReferrerUserID, s.UserID)
		if err := UpsertUserChallenge(ctx, tx,
			p.ID, specifier, *extra.ReferrerUserID, 1, 1, amount,
		); err != nil {
			return fmt.Errorf("upsert: %w", err)
		}
	}
	if maxID > prev {
		if err := writeCheckpointInt(ctx, tx, cpName, maxID); err != nil {
			return err
		}
	}
	return nil
}

// ReferredProcessor implements challenge "rd" — referred user earns
// once for being referred. Same signal type as r/rv ("referral"),
// different processor — they checkpoint independently.
type ReferredProcessor struct{}

func (p *ReferredProcessor) ChallengeID() string { return "rd" }

func (p *ReferredProcessor) Reconcile(ctx context.Context, tx pgx.Tx) error {
	c, ok, err := LoadChallenge(ctx, tx, p.ChallengeID())
	if err != nil {
		return fmt.Errorf("load challenge: %w", err)
	}
	if !ok || !c.Active {
		return nil
	}
	amount := c.AmountInt()
	cpName := signalsCheckpointName(p.ChallengeID(), "referral")
	prev, err := readCheckpointInt(ctx, tx, cpName)
	if err != nil {
		return err
	}
	signals, maxID, err := readSignalsSince(ctx, tx, "referral", prev, 5000)
	if err != nil {
		return err
	}
	for _, s := range signals {
		var extra struct {
			ReferrerUserID *int64 `json:"referrer_user_id"`
		}
		if err := decodeSignalExtra(s.ExtraJSON, &extra); err != nil {
			return fmt.Errorf("decode: %w", err)
		}
		referrer := int64(0)
		if extra.ReferrerUserID != nil {
			referrer = *extra.ReferrerUserID
		}
		specifier := fmt.Sprintf("%x:%x", s.UserID, referrer)
		if err := UpsertUserChallenge(ctx, tx,
			p.ChallengeID(), specifier, s.UserID, 1, 1, amount,
		); err != nil {
			return fmt.Errorf("upsert: %w", err)
		}
	}
	if maxID > prev {
		if err := writeCheckpointInt(ctx, tx, cpName, maxID); err != nil {
			return err
		}
	}
	return nil
}
