package challenges

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// ProfileCompletionProcessor implements challenge "p" — 7 boolean steps:
//
//	profile_description (bio set)
//	profile_name (name set)
//	profile_picture (profile_picture or profile_picture_sizes set)
//	profile_cover_photo (cover_photo or cover_photo_sizes set)
//	follows (>= 5 follows)
//	reposts (>= 1 repost)
//	favorites (>= 1 favorite)
//
// State lives in the `challenge_profile_completion` table; user_challenges'
// current_step_count is the sum of the 7 booleans.
//
// Mirrors apps/packages/discovery-provider/src/challenges/profile_challenge.py.
//
// Incremental: rather than rescanning users/follows/reposts/saves every tick
// (a full recompute timed out past 90s against prod), we only recompute users
// touched since the last checkpoint. See incremental.go.
type ProfileCompletionProcessor struct{}

func (p *ProfileCompletionProcessor) ChallengeID() string { return "p" }

const (
	profileFollowThreshold   = 5
	profileRepostThreshold   = 1
	profileFavoriteThreshold = 1

	profileCheckpoint = "challenges:p:last_blocknumber"
)

// profileDirtySQL returns (user_id, blocknumber) for every user whose profile,
// follow, repost, or favorite state changed since the checkpoint. Each source
// row maps to the user it belongs to; the union is index-scanned on blocknumber.
const profileDirtySQL = `
	SELECT user_id, blocknumber FROM (
		SELECT follower_user_id AS user_id, blocknumber FROM follows WHERE blocknumber > $1
		UNION ALL SELECT user_id, blocknumber FROM reposts WHERE blocknumber > $1
		UNION ALL SELECT user_id, blocknumber FROM saves   WHERE blocknumber > $1
		UNION ALL SELECT user_id, blocknumber FROM users   WHERE blocknumber > $1
	) s
	ORDER BY blocknumber ASC
	LIMIT $2
`

func (p *ProfileCompletionProcessor) Reconcile(ctx context.Context, tx pgx.Tx) error {
	c, ok, err := LoadChallenge(ctx, tx, p.ChallengeID())
	if err != nil {
		return fmt.Errorf("load challenge: %w", err)
	}
	if !ok || !c.Active || c.StepCount == nil {
		return nil
	}
	stepCount := *c.StepCount // should be 7
	amount := c.AmountInt()

	return reconcileIncrementalUsers(ctx, tx, profileCheckpoint, profileDirtySQL,
		func(ctx context.Context, tx pgx.Tx, userIDs []int64) error {
			return p.recompute(ctx, tx, userIDs, stepCount, amount)
		})
}

// recompute re-derives the seven profile steps for the given user ids and
// upserts both the per-challenge state table and user_challenges.
//
// The follow/repost/favorite steps are pure thresholds (>=5, >=1, >=1), so we
// cap the scans: the follow count stops at the threshold (LIMIT) and the
// repost/favorite checks are EXISTS. That turns each per-user probe from an
// unbounded COUNT(*) over a possibly-huge history into an O(threshold) index
// lookup — the difference between the 90s full-table timeout and ~ms here.
func (p *ProfileCompletionProcessor) recompute(ctx context.Context, tx pgx.Tx, userIDs []int64, stepCount, amount int32) error {
	rows, err := tx.Query(ctx, `
		SELECT u.user_id,
		       (u.bio IS NOT NULL)  AS f_bio,
		       (u.name IS NOT NULL) AS f_name,
		       (u.profile_picture IS NOT NULL OR u.profile_picture_sizes IS NOT NULL) AS f_picture,
		       (u.cover_photo IS NOT NULL OR u.cover_photo_sizes IS NOT NULL)         AS f_cover,
		       (fc.cnt >= $2) AS f_follows,
		       EXISTS (SELECT 1 FROM reposts r WHERE r.user_id = u.user_id AND r.is_current AND NOT r.is_delete) AS f_reposts,
		       EXISTS (SELECT 1 FROM saves   sv WHERE sv.user_id = u.user_id AND sv.is_current AND NOT sv.is_delete) AS f_favorites
		FROM users u
		LEFT JOIN LATERAL (
			SELECT count(*) AS cnt
			FROM (
				SELECT 1 FROM follows f
				WHERE f.follower_user_id = u.user_id AND f.is_current AND NOT f.is_delete
				LIMIT $2
			) z
		) fc ON true
		WHERE u.user_id = ANY($1)
		  AND u.is_current = true
		  AND u.handle_lc IS NOT NULL
		  AND u.is_deactivated = false
	`, userIDs, profileFollowThreshold)
	if err != nil {
		return fmt.Errorf("scan profile users: %w", err)
	}
	type pcRow struct {
		userID                                                        int64
		fBio, fName, fPicture, fCover, fFollows, fReposts, fFavorites bool
	}
	var results []pcRow
	for rows.Next() {
		var r pcRow
		if err := rows.Scan(&r.userID,
			&r.fBio, &r.fName, &r.fPicture, &r.fCover,
			&r.fFollows, &r.fReposts, &r.fFavorites); err != nil {
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
		steps := b2i(r.fBio) + b2i(r.fName) + b2i(r.fPicture) + b2i(r.fCover) +
			b2i(r.fFollows) + b2i(r.fReposts) + b2i(r.fFavorites)

		// Upsert the per-challenge state table first so the booleans are
		// queryable (apps tools read this for client display).
		if _, err := tx.Exec(ctx, `
			INSERT INTO challenge_profile_completion
				(user_id, profile_description, profile_name, profile_picture, profile_cover_photo,
				 follows, reposts, favorites)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
			ON CONFLICT (user_id) DO UPDATE SET
				profile_description = EXCLUDED.profile_description,
				profile_name = EXCLUDED.profile_name,
				profile_picture = EXCLUDED.profile_picture,
				profile_cover_photo = EXCLUDED.profile_cover_photo,
				follows = EXCLUDED.follows,
				reposts = EXCLUDED.reposts,
				favorites = EXCLUDED.favorites
		`, r.userID, r.fBio, r.fName, r.fPicture, r.fCover, r.fFollows, r.fReposts, r.fFavorites); err != nil {
			return fmt.Errorf("upsert profile_completion: %w", err)
		}
		if err := UpsertUserChallenge(ctx, tx,
			p.ChallengeID(), SpecifierFromUserID(r.userID),
			r.userID, steps, stepCount, amount,
		); err != nil {
			return fmt.Errorf("upsert user_challenge: %w", err)
		}
	}
	return nil
}

func b2i(b bool) int32 {
	if b {
		return 1
	}
	return 0
}
