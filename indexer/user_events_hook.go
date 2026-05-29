package indexer

import (
	"context"
	"errors"

	em "github.com/OpenAudio/go-openaudio/pkg/etl/processors/entity_manager"
	"github.com/jackc/pgx/v5"
	"go.uber.org/zap"
)

// newUserEventsHook returns a pkg/etl post-hook that records the
// `events` sub-object of a User Create/Update tx's metadata into the
// user_events table (is_mobile_user, referrer).
//
// This is the on-chain source for the mobile-install ("m") and referral
// ("r"/"rv"/"rd") challenges. The client encodes these as fields in the
// user's profile metadata (events.is_mobile_user / events.referrer) on a
// normal User tx; we index them into user_events so the challenge job's
// Reconcile step can recompute completions from indexed state — same
// poll-based pattern as the on-chain-derived Phase 1/2 challenges.
//
// Mirrors the legacy Python discovery-provider's update_user_events
// (apps entity_manager/entities/user.py): is_mobile_user is sticky (once
// true, stays true) and referrer is set once (only when currently unset
// and not self-referral).
//
// Registered on both User Create and User Update — the fields can arrive
// on either. Hook errors are logged but not propagated (em.PostHook
// contract); the user row still commits.
func newUserEventsHook(logger *zap.Logger) em.PostHook {
	hookLogger := logger.Named("UserEventsHook")
	return func(ctx context.Context, params *em.Params) error {
		if err := indexUserEvents(ctx, params); err != nil {
			hookLogger.Warn("failed to index user_events",
				zap.Int64("user_id", params.EntityID),
				zap.Error(err))
		}
		return nil
	}
}

func indexUserEvents(ctx context.Context, params *em.Params) error {
	events, ok := params.Metadata["events"].(map[string]any)
	if !ok || len(events) == 0 {
		return nil // no events sub-object on this tx
	}
	userID := params.EntityID

	// Incoming values from metadata.events.
	incomingMobile := false
	if v, ok := events["is_mobile_user"].(bool); ok {
		incomingMobile = v
	}
	var incomingReferrer *int64
	switch r := events["referrer"].(type) {
	case float64: // JSON numbers decode to float64
		if id := int64(r); id > 0 && id != userID {
			incomingReferrer = &id
		}
	case int64:
		if r > 0 && r != userID {
			incomingReferrer = &r
		}
	}
	if !incomingMobile && incomingReferrer == nil {
		return nil // nothing relevant in events
	}

	// Current state for this user (one is_current row, Python-style).
	var curMobile bool
	var curReferrer *int64
	err := params.DBTX.QueryRow(ctx,
		`SELECT is_mobile_user, referrer FROM user_events WHERE user_id = $1 AND is_current = true`,
		userID,
	).Scan(&curMobile, &curReferrer)
	exists := err == nil
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return err
	}

	// Merge: is_mobile_user is sticky; referrer is set-once.
	newMobile := curMobile || incomingMobile
	newReferrer := curReferrer
	if newReferrer == nil && incomingReferrer != nil {
		newReferrer = incomingReferrer
	}

	// No-op if nothing changed (keeps the table from churning on repeat
	// profile updates that re-send the same events).
	if exists && newMobile == curMobile && int64PtrEq(newReferrer, curReferrer) {
		return nil
	}

	// Demote the prior current row and insert the merged one atomically.
	// user_events has no unique constraint on user_id (Python keeps
	// historical rows), so we version with is_current rather than upsert.
	_, err = params.DBTX.Exec(ctx, `
		WITH demoted AS (
			UPDATE user_events SET is_current = false
			WHERE user_id = $1 AND is_current = true
		)
		INSERT INTO user_events (user_id, is_mobile_user, referrer, is_current, blocknumber, blockhash)
		VALUES ($1, $2, $3, true, $4, $5)
	`, userID, newMobile, newReferrer, params.BlockNumber, params.BlockHash)
	return err
}

func int64PtrEq(a, b *int64) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}
