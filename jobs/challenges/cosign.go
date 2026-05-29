package challenges

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// CosignProcessor implements challenge "cs" — a verified parent-track
// owner saving or reposting a remix of their own track earns the
// *remixer* the cosign reward. Mirrors apps' dispatch in
// social_features.py:135.
//
// Specifier (matches apps):
//
//	<hex_parent_track_owner_id>:<hex_remix_track_id>
//
// Cap: max 5 cosigns per parent-owner per rolling 30 days (apps'
// MAX_COSIGNS_PER_MONTH). We enforce that by counting existing
// user_challenges rows with the same specifier prefix in the last 30
// days and skipping insertion when the cap is reached.
//
// Incremental: the trigger is a save/repost *action* by a parent owner, so we
// checkpoint on the reposts+saves blocknumber and only look at actions since
// the last tick — joining each back to remixes to see if it cosigns one of the
// actor's own tracks. We advance the checkpoint over every scanned action (not
// just cosign-producing ones) so non-cosign social activity isn't rescanned.
//
// Note: catalog row for "cs" is currently inactive in apps. Reconcile
// short-circuits when inactive — code is in place for when it's enabled.
type CosignProcessor struct{}

func (p *CosignProcessor) ChallengeID() string { return "cs" }

const (
	cosignCapPerMonth = 5
	cosignCheckpoint  = "challenges:cs:last_blocknumber"
)

// cosignDirtySQL surfaces every track save/repost since the checkpoint and
// LEFT JOINs the cosign chain. is_cosign is true only when the actor owns the
// current parent track of a current remix and is verified — i.e. a real
// cosign. Non-cosign rows still come back so we can advance the high-water mark
// past them.
const cosignDirtySQL = `
	SELECT da.blocknumber,
	       da.actor_id,
	       child.owner_id AS remixer_id,
	       r.child_track_id,
	       (parent.track_id IS NOT NULL
	        AND child.track_id IS NOT NULL
	        AND u.user_id IS NOT NULL) AS is_cosign
	FROM (
		SELECT user_id AS actor_id, repost_item_id AS track_id, blocknumber
		FROM reposts
		WHERE blocknumber > $1 AND is_current AND NOT is_delete AND repost_type = 'track'
		UNION ALL
		SELECT user_id AS actor_id, save_item_id AS track_id, blocknumber
		FROM saves
		WHERE blocknumber > $1 AND is_current AND NOT is_delete AND save_type = 'track'
	) da
	LEFT JOIN remixes r ON r.child_track_id = da.track_id
	LEFT JOIN tracks parent ON parent.track_id = r.parent_track_id
		AND parent.is_current = true AND parent.owner_id = da.actor_id
	LEFT JOIN tracks child ON child.track_id = r.child_track_id
		AND child.is_current = true
	LEFT JOIN users u ON u.user_id = da.actor_id
		AND u.is_current = true AND u.is_verified = true
	ORDER BY da.blocknumber ASC
	LIMIT $2
`

func (p *CosignProcessor) Reconcile(ctx context.Context, tx pgx.Tx) error {
	c, ok, err := LoadChallenge(ctx, tx, p.ChallengeID())
	if err != nil {
		return fmt.Errorf("load challenge: %w", err)
	}
	if !ok || !c.Active {
		return nil
	}
	amount := c.AmountInt()

	prev, err := readCheckpointInt(ctx, tx, cosignCheckpoint)
	if err != nil {
		return fmt.Errorf("read checkpoint: %w", err)
	}

	rows, err := tx.Query(ctx, cosignDirtySQL, prev, dirtyScanBatch)
	if err != nil {
		return fmt.Errorf("scan cosign actions: %w", err)
	}
	type cosignRow struct {
		parentOwnerID int64
		remixerID     int64
		remixTrackID  int64
	}
	var candidates []cosignRow
	scanned := 0
	maxBn := prev
	for rows.Next() {
		var bn, actorID int64
		var remixerID, remixTrackID *int64
		var isCosign bool
		if err := rows.Scan(&bn, &actorID, &remixerID, &remixTrackID, &isCosign); err != nil {
			rows.Close()
			return err
		}
		scanned++
		if bn > maxBn {
			maxBn = bn
		}
		if isCosign && remixerID != nil && remixTrackID != nil {
			candidates = append(candidates, cosignRow{
				parentOwnerID: actorID,
				remixerID:     *remixerID,
				remixTrackID:  *remixTrackID,
			})
		}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}
	if scanned == 0 {
		return nil
	}

	for _, r := range candidates {
		specifier := fmt.Sprintf("%x:%x", r.parentOwnerID, r.remixTrackID)
		specifierPrefix := fmt.Sprintf("%x:", r.parentOwnerID)

		// If this specifier already exists, the upsert below is a no-op, so
		// don't let it consume a cap slot.
		var alreadyHave bool
		if err := tx.QueryRow(ctx, `
			SELECT EXISTS (SELECT 1 FROM user_challenges
			               WHERE challenge_id = 'cs' AND specifier = $1)
		`, specifier).Scan(&alreadyHave); err != nil {
			return err
		}
		if alreadyHave {
			continue
		}

		// Per-parent-owner 30-day cap. Counts ALL cosign user_challenges for
		// this parent-owner in the last 30 days; matches apps' behavior (the
		// cap is on the *cosigner*, not the recipient). Rows inserted earlier
		// in this same tx are visible here, so the cap holds within a batch.
		var cosignsLastMonth int
		if err := tx.QueryRow(ctx, `
			SELECT COUNT(*) FROM user_challenges
			WHERE challenge_id = 'cs'
			  AND specifier LIKE $1
			  AND created_at >= now() - INTERVAL '30 days'
		`, specifierPrefix+"%").Scan(&cosignsLastMonth); err != nil {
			return fmt.Errorf("count recent cosigns: %w", err)
		}
		if cosignsLastMonth >= cosignCapPerMonth {
			continue
		}

		if err := UpsertUserChallenge(ctx, tx,
			p.ChallengeID(), specifier, r.remixerID, 1, 1, amount,
		); err != nil {
			return fmt.Errorf("upsert: %w", err)
		}
	}

	newMax := highWaterMark(prev, maxBn, scanned)
	if newMax > prev {
		if err := writeCheckpointInt(ctx, tx, cosignCheckpoint, newMax); err != nil {
			return fmt.Errorf("save checkpoint: %w", err)
		}
	}
	return nil
}
