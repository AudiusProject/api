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
type ProfileCompletionProcessor struct{}

func (p *ProfileCompletionProcessor) ChallengeID() string { return "p" }

const (
	profileFollowThreshold   = 5
	profileRepostThreshold   = 1
	profileFavoriteThreshold = 1
)

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

	// Recompute every step from scratch for users with any in-flight
	// or completable progress. We use a single CTE-driven query that
	// returns one row per user with the seven booleans.
	//
	// We only consider users with handle_lc set (i.e. real accounts),
	// matching apps' downstream behavior — anonymous/guest users don't
	// earn challenges.
	rows, err := tx.Query(ctx, `
		WITH active_users AS (
			SELECT user_id, bio, name,
			       profile_picture, profile_picture_sizes,
			       cover_photo, cover_photo_sizes
			FROM users
			WHERE is_current = true
			  AND handle_lc IS NOT NULL
			  AND is_deactivated = false
		),
		follow_counts AS (
			SELECT follower_user_id AS user_id, COUNT(*) AS n
			FROM follows
			WHERE is_current = true AND is_delete = false
			GROUP BY follower_user_id
		),
		repost_counts AS (
			SELECT user_id, COUNT(*) AS n
			FROM reposts
			WHERE is_current = true AND is_delete = false
			GROUP BY user_id
		),
		save_counts AS (
			SELECT user_id, COUNT(*) AS n
			FROM saves
			WHERE is_current = true AND is_delete = false
			GROUP BY user_id
		)
		SELECT u.user_id,
		       (u.bio IS NOT NULL)::int +
		       (u.name IS NOT NULL)::int +
		       ((u.profile_picture IS NOT NULL OR u.profile_picture_sizes IS NOT NULL))::int +
		       ((u.cover_photo IS NOT NULL OR u.cover_photo_sizes IS NOT NULL))::int +
		       (COALESCE(fc.n, 0) >= $1)::int +
		       (COALESCE(rc.n, 0) >= $2)::int +
		       (COALESCE(sc.n, 0) >= $3)::int AS steps,
		       (u.bio IS NOT NULL) AS f_bio,
		       (u.name IS NOT NULL) AS f_name,
		       (u.profile_picture IS NOT NULL OR u.profile_picture_sizes IS NOT NULL) AS f_picture,
		       (u.cover_photo IS NOT NULL OR u.cover_photo_sizes IS NOT NULL) AS f_cover,
		       (COALESCE(fc.n, 0) >= $1) AS f_follows,
		       (COALESCE(rc.n, 0) >= $2) AS f_reposts,
		       (COALESCE(sc.n, 0) >= $3) AS f_favorites
		FROM active_users u
		LEFT JOIN follow_counts fc ON fc.user_id = u.user_id
		LEFT JOIN repost_counts rc ON rc.user_id = u.user_id
		LEFT JOIN save_counts sc ON sc.user_id = u.user_id
		WHERE
		    -- Only touch users with at least one step OR an existing in-progress row.
		    u.bio IS NOT NULL OR u.name IS NOT NULL
		    OR u.profile_picture IS NOT NULL OR u.profile_picture_sizes IS NOT NULL
		    OR u.cover_photo IS NOT NULL OR u.cover_photo_sizes IS NOT NULL
		    OR COALESCE(fc.n, 0) >= $1
		    OR COALESCE(rc.n, 0) >= $2
		    OR COALESCE(sc.n, 0) >= $3
	`, profileFollowThreshold, profileRepostThreshold, profileFavoriteThreshold)
	if err != nil {
		return fmt.Errorf("scan profile users: %w", err)
	}
	type pcRow struct {
		userID                                                          int64
		steps                                                           int32
		fBio, fName, fPicture, fCover, fFollows, fReposts, fFavorites bool
	}
	var results []pcRow
	for rows.Next() {
		var r pcRow
		if err := rows.Scan(&r.userID, &r.steps,
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
			r.userID, r.steps, stepCount, amount,
		); err != nil {
			return fmt.Errorf("upsert user_challenge: %w", err)
		}
	}
	return nil
}
