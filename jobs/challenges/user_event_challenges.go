// Package challenges, user_events-sourced processors.
//
// The mobile-install ("m") and referral ("r"/"rv"/"rd") challenges are
// driven by the user_events table, which the indexer populates from the
// `events` object in user-profile metadata (is_mobile_user, referrer) —
// see indexer/user_events_hook.go. Each processor recomputes completions
// from the current user_events state on every Reconcile (no checkpoint),
// matching the on-chain-derived Phase 1/2 processors.
package challenges

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// MobileInstallProcessor implements challenge "m" — boolean reward when a
// user's profile reports is_mobile_user (events.is_mobile_user=true).
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

	rows, err := tx.Query(ctx, `
		SELECT user_id FROM user_events
		WHERE is_current = true AND is_mobile_user = true
	`)
	if err != nil {
		return fmt.Errorf("scan user_events: %w", err)
	}
	var userIDs []int64
	for rows.Next() {
		var uid int64
		if err := rows.Scan(&uid); err != nil {
			rows.Close()
			return err
		}
		userIDs = append(userIDs, uid)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}

	for _, uid := range userIDs {
		if err := UpsertUserChallenge(ctx, tx,
			p.ChallengeID(), SpecifierFromUserID(uid), uid, 1, 1, amount,
		); err != nil {
			return fmt.Errorf("upsert: %w", err)
		}
	}
	return nil
}

// ReferralProcessor implements challenges "r" / "rv" — the referrer earns
// when another user records them as referrer (events.referrer). The split
// gates on the referrer's verification:
//
//	r:  referrer NOT verified
//	rv: referrer IS verified
//
// (Python only ever recorded verified referrers; sourcing from
// user_events here lets api/ award the unverified "r" tier too, per the
// Phase 3 challenge catalog.)
type ReferralProcessor struct {
	ID       string
	Verified bool
}

func NewReferralProcessor() Processor         { return &ReferralProcessor{ID: "r", Verified: false} }
func NewVerifiedReferralProcessor() Processor { return &ReferralProcessor{ID: "rv", Verified: true} }

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

	rows, err := tx.Query(ctx, `
		SELECT ue.user_id, ue.referrer
		FROM user_events ue
		JOIN users u ON u.user_id = ue.referrer AND u.is_current = true
		WHERE ue.is_current = true
		  AND ue.referrer IS NOT NULL
		  AND COALESCE(u.is_verified, false) = $1
	`, p.Verified)
	if err != nil {
		return fmt.Errorf("scan referrals: %w", err)
	}
	type ref struct{ referred, referrer int64 }
	var refs []ref
	for rows.Next() {
		var r ref
		if err := rows.Scan(&r.referred, &r.referrer); err != nil {
			rows.Close()
			return err
		}
		refs = append(refs, r)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}

	for _, r := range refs {
		// Specifier: <hex_referrer>:<hex_referred> — reward target is the referrer.
		specifier := fmt.Sprintf("%x:%x", r.referrer, r.referred)
		if err := UpsertUserChallenge(ctx, tx,
			p.ID, specifier, r.referrer, 1, 1, amount,
		); err != nil {
			return fmt.Errorf("upsert: %w", err)
		}
	}
	return nil
}

// ReferredProcessor implements challenge "rd" — the referred user earns
// once for having a referrer recorded (regardless of referrer
// verification). Sourced from user_events.
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

	rows, err := tx.Query(ctx, `
		SELECT user_id, referrer FROM user_events
		WHERE is_current = true AND referrer IS NOT NULL
	`)
	if err != nil {
		return fmt.Errorf("scan referrals: %w", err)
	}
	type ref struct{ referred, referrer int64 }
	var refs []ref
	for rows.Next() {
		var r ref
		if err := rows.Scan(&r.referred, &r.referrer); err != nil {
			rows.Close()
			return err
		}
		refs = append(refs, r)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}

	for _, r := range refs {
		// Specifier: <hex_referred>:<hex_referrer> — reward target is the referred user.
		specifier := fmt.Sprintf("%x:%x", r.referred, r.referrer)
		if err := UpsertUserChallenge(ctx, tx,
			p.ChallengeID(), specifier, r.referred, 1, 1, amount,
		); err != nil {
			return fmt.Errorf("upsert: %w", err)
		}
	}
	return nil
}
