// Package challenges, user_events-sourced processors.
//
// The mobile-install ("m") and referral ("r"/"rv"/"rd") challenges are
// driven by the user_events table, which the indexer populates from the
// `events` object in user-profile metadata (is_mobile_user, referrer) —
// see indexer/user_events_hook.go.
//
// Incremental: like #875's other dirty-set processors, each tick only
// rescans user_events rows whose blocknumber moved past a per-processor
// checkpoint, then re-derives state for just the affected users. Three
// supporting migrations land alongside this file:
//
//   - 0209 adds a btree on user_events(blocknumber) so the dirty scan is
//     an index range read instead of a 2.3M-row seq-scan.
//   - 0210 adds a partial GIN on notification(user_ids) WHERE
//     type='reward_in_cooldown' — the on_user_challenge trigger's
//     cooldown-window check fires on every is_complete=true upsert for
//     challenges with cooldown_days>0 (which all four Phase 3 challenges
//     have), and without that index it scans the full 8GB notification
//     table per call.
//   - 0211 seeds the four checkpoints to the current max
//     user_events.blocknumber so prod starts "from now" and skips
//     re-deriving completions over 2.3M historical rows on first run.
//     Python already populated user_challenges; upserts are idempotent.
//
// Caveat: the dirty scan keys on user_events.blocknumber only, so a
// verification flip on an existing referrer's users row (`is_verified`
// goes true) is NOT picked up by the r/rv processors until the *referred*
// user's user_events row changes again. Verification changes are rare and
// the old full-scan code caught them only on its next tick; this matches
// #875's other processors which similarly key on a single source.
package challenges

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// userEventReferralDirtySQL surfaces user_events rows whose blocknumber moved
// past the checkpoint and currently carry a referrer. Shared by the r, rv, and
// rd processors — the dirty source is the same; recompute partitions the
// recipient (referrer vs referred) and the verification gate.
//
// The user_id column here is the *referred* user (the user_events row that
// names a referrer); recompute resolves the (referrer, referred) pairs.
const userEventReferralDirtySQL = `
	SELECT user_id, blocknumber FROM user_events
	WHERE blocknumber > $1
	  AND is_current = true
	  AND referrer IS NOT NULL
	ORDER BY blocknumber ASC
	LIMIT $2
`

// MobileInstallProcessor — challenge "m". Boolean reward when a user's
// profile reports is_mobile_user=true (events.is_mobile_user).
type MobileInstallProcessor struct{}

func (p *MobileInstallProcessor) ChallengeID() string { return "m" }

const mobileInstallCheckpoint = "challenges:m:last_blocknumber"

const mobileInstallDirtySQL = `
	SELECT user_id, blocknumber FROM user_events
	WHERE blocknumber > $1
	  AND is_current = true
	  AND is_mobile_user = true
	ORDER BY blocknumber ASC
	LIMIT $2
`

func (p *MobileInstallProcessor) Reconcile(ctx context.Context, tx pgx.Tx) error {
	c, ok, err := LoadChallenge(ctx, tx, p.ChallengeID())
	if err != nil {
		return fmt.Errorf("load challenge: %w", err)
	}
	if !ok || !c.Active {
		return nil
	}
	amount := c.AmountInt()

	return reconcileIncrementalUsers(ctx, tx, mobileInstallCheckpoint, mobileInstallDirtySQL,
		func(ctx context.Context, tx pgx.Tx, userIDs []int64) error {
			return p.recompute(ctx, tx, userIDs, amount)
		})
}

func (p *MobileInstallProcessor) recompute(ctx context.Context, tx pgx.Tx, userIDs []int64, amount int32) error {
	// Re-verify each candidate still has a mobile-flagged is_current row.
	// Within the same tx this is mostly belt-and-suspenders, but the EXISTS
	// pattern mirrors first_playlist.go and stays robust if the dirty scan
	// ever loosens its filter.
	rows, err := tx.Query(ctx, `
		SELECT x.user_id
		FROM unnest($1::bigint[]) AS x(user_id)
		WHERE EXISTS (
			SELECT 1 FROM user_events ue
			WHERE ue.user_id = x.user_id
			  AND ue.is_current = true
			  AND ue.is_mobile_user = true
		)
	`, userIDs)
	if err != nil {
		return fmt.Errorf("scan user_events: %w", err)
	}
	var qualifying []int64
	for rows.Next() {
		var uid int64
		if err := rows.Scan(&uid); err != nil {
			rows.Close()
			return err
		}
		qualifying = append(qualifying, uid)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}

	for _, uid := range qualifying {
		if err := UpsertUserChallenge(ctx, tx,
			p.ChallengeID(), SpecifierFromUserID(uid), uid, 1, 1, amount,
		); err != nil {
			return fmt.Errorf("upsert: %w", err)
		}
	}
	return nil
}

// ReferralProcessor — challenges "r" / "rv". The referrer earns when
// another user records them as referrer; the split gates on the referrer's
// verification:
//
//	r:  referrer NOT verified
//	rv: referrer IS verified
//
// Python only ever recorded verified referrers; sourcing from user_events
// here lets api/ award the unverified "r" tier too per the Phase 3 catalog.
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
	checkpoint := fmt.Sprintf("challenges:%s:last_blocknumber", p.ID)

	return reconcileIncrementalUsers(ctx, tx, checkpoint, userEventReferralDirtySQL,
		func(ctx context.Context, tx pgx.Tx, referredIDs []int64) error {
			return p.recompute(ctx, tx, referredIDs, amount)
		})
}

func (p *ReferralProcessor) recompute(ctx context.Context, tx pgx.Tx, referredIDs []int64, amount int32) error {
	rows, err := tx.Query(ctx, `
		SELECT ue.user_id, ue.referrer
		FROM user_events ue
		JOIN users u ON u.user_id = ue.referrer AND u.is_current = true
		WHERE ue.user_id = ANY($1::bigint[])
		  AND ue.is_current = true
		  AND ue.referrer IS NOT NULL
		  AND COALESCE(u.is_verified, false) = $2
	`, referredIDs, p.Verified)
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
		// Specifier: <hex_referrer>:<hex_referred> — recipient is the referrer.
		specifier := fmt.Sprintf("%x:%x", r.referrer, r.referred)
		if err := UpsertUserChallenge(ctx, tx,
			p.ID, specifier, r.referrer, 1, 1, amount,
		); err != nil {
			return fmt.Errorf("upsert: %w", err)
		}
	}
	return nil
}

// ReferredProcessor — challenge "rd". The referred user earns once for
// having a referrer recorded (regardless of referrer verification).
type ReferredProcessor struct{}

func (p *ReferredProcessor) ChallengeID() string { return "rd" }

const referredCheckpoint = "challenges:rd:last_blocknumber"

func (p *ReferredProcessor) Reconcile(ctx context.Context, tx pgx.Tx) error {
	c, ok, err := LoadChallenge(ctx, tx, p.ChallengeID())
	if err != nil {
		return fmt.Errorf("load challenge: %w", err)
	}
	if !ok || !c.Active {
		return nil
	}
	amount := c.AmountInt()

	return reconcileIncrementalUsers(ctx, tx, referredCheckpoint, userEventReferralDirtySQL,
		func(ctx context.Context, tx pgx.Tx, referredIDs []int64) error {
			return p.recompute(ctx, tx, referredIDs, amount)
		})
}

func (p *ReferredProcessor) recompute(ctx context.Context, tx pgx.Tx, referredIDs []int64, amount int32) error {
	rows, err := tx.Query(ctx, `
		SELECT user_id, referrer FROM user_events
		WHERE user_id = ANY($1::bigint[])
		  AND is_current = true
		  AND referrer IS NOT NULL
	`, referredIDs)
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
		// Specifier: <hex_referred>:<hex_referrer> — recipient is the referred user.
		specifier := fmt.Sprintf("%x:%x", r.referred, r.referrer)
		if err := UpsertUserChallenge(ctx, tx,
			p.ChallengeID(), specifier, r.referred, 1, 1, amount,
		); err != nil {
			return fmt.Errorf("upsert: %w", err)
		}
	}
	return nil
}
