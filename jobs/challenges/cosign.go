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
// Note: catalog row for "cs" is currently inactive in apps. Reconcile
// short-circuits when inactive — code is in place for when it's enabled.
type CosignProcessor struct{}

func (p *CosignProcessor) ChallengeID() string { return "cs" }

const cosignCapPerMonth = 5

func (p *CosignProcessor) Reconcile(ctx context.Context, tx pgx.Tx) error {
	c, ok, err := LoadChallenge(ctx, tx, p.ChallengeID())
	if err != nil {
		return fmt.Errorf("load challenge: %w", err)
	}
	if !ok || !c.Active {
		return nil
	}
	amount := c.AmountInt()

	// Find (parent_owner, remix_owner, remix_track_id, cosign_at) tuples
	// where the parent owner has saved or reposted the remix and the
	// parent owner is verified. Use the *earliest* action timestamp per
	// (parent_owner, remix) so the cooldown calc is stable.
	rows, err := tx.Query(ctx, `
		WITH cosign_actions AS (
		    SELECT r.parent_track_id,
		           r.child_track_id,
		           parent.owner_id   AS parent_owner_id,
		           child.owner_id    AS remixer_user_id,
		           MIN(a.created_at) AS cosign_at
		    FROM remixes r
		    JOIN tracks parent ON parent.track_id = r.parent_track_id
		         AND parent.is_current = true
		    JOIN tracks child  ON child.track_id  = r.child_track_id
		         AND child.is_current = true
		    JOIN users u ON u.user_id = parent.owner_id
		         AND u.is_current = true
		         AND u.is_verified = true
		    JOIN (
		        SELECT user_id, repost_item_id AS item_id, created_at
		        FROM reposts
		        WHERE is_current = true AND is_delete = false AND repost_type = 'track'
		        UNION ALL
		        SELECT user_id, save_item_id AS item_id, created_at
		        FROM saves
		        WHERE is_current = true AND is_delete = false AND save_type = 'track'
		    ) a ON a.user_id = parent.owner_id AND a.item_id = r.child_track_id
		    GROUP BY r.parent_track_id, r.child_track_id,
		             parent.owner_id, child.owner_id
		)
		SELECT parent_owner_id, remixer_user_id, child_track_id, cosign_at
		FROM cosign_actions
		ORDER BY cosign_at ASC
	`)
	if err != nil {
		return fmt.Errorf("scan cosigns: %w", err)
	}
	type cosignRow struct {
		parentOwnerID int64
		remixerID     int64
		remixTrackID  int64
	}
	var results []cosignRow
	for rows.Next() {
		var r cosignRow
		var skipTime any // we don't need cosign_at after sort
		if err := rows.Scan(&r.parentOwnerID, &r.remixerID, &r.remixTrackID, &skipTime); err != nil {
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
		specifier := fmt.Sprintf("%x:%x", r.parentOwnerID, r.remixTrackID)
		specifierPrefix := fmt.Sprintf("%x:", r.parentOwnerID)

		// Per-parent-owner 30-day cap. Counts ALL cosign user_challenges
		// for this parent-owner in the last 30 days; matches apps'
		// behavior (the cap is on the *cosigner*, not the recipient).
		var cosignsLastMonth int
		if err := tx.QueryRow(ctx, `
			SELECT COUNT(*) FROM user_challenges
			WHERE challenge_id = 'cs'
			  AND specifier LIKE $1
			  AND created_at >= now() - INTERVAL '30 days'
		`, specifierPrefix+"%").Scan(&cosignsLastMonth); err != nil {
			return fmt.Errorf("count recent cosigns: %w", err)
		}

		// If this specifier already exists, the upsert below is a no-op,
		// so we shouldn't gate on the cap for already-minted rows.
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
		if cosignsLastMonth >= cosignCapPerMonth {
			continue
		}

		if err := UpsertUserChallenge(ctx, tx,
			p.ChallengeID(), specifier, r.remixerID, 1, 1, amount,
		); err != nil {
			return fmt.Errorf("upsert: %w", err)
		}
	}
	return nil
}
