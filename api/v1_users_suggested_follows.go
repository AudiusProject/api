package api

import (
	"context"
	"fmt"

	"api.audius.co/api/dbv1"
	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"
)

type GetUsersSuggestedFollowsParams struct {
	Limit  int `query:"limit" default:"10" validate:"min=1,max=100"`
	Offset int `query:"offset" default:"0" validate:"min=0"`
}

const (
	// Only the most recent N favorites and N reposts are considered. A user's
	// engagement history is unbounded, and everything downstream of it joins
	// per-row, so this caps the worst case for heavy users. Recent engagement
	// is also the better signal, so the cap costs little.
	//
	// Note this cap, not the decay below, is the effective window for anyone
	// with a large library: a user who favorites 50 tracks a day is scored on
	// their last ~40 days, well inside the decay's range. For everyone else the
	// cap never binds and the decay does the shaping.
	//
	// Lowering it further is tempting for latency but trades against fill rate:
	// the cap bounds the candidate pool *before* already-followed artists are
	// filtered out, so a user who follows most of the artists they recently
	// engaged with gets a short list. saves_user_created_at_active_idx
	// (migration 0239) removes the reason to make that trade.
	suggestedFollowsEngagementCap = 2000

	// Engagement weight decays with e^(-age/tau). At tau = 180 days a favorite
	// from six months ago counts ~37% of one from today, so long-dormant taste
	// still contributes but does not outrank what the user likes now.
	suggestedFollowsDecaySeconds = 180 * 24 * 60 * 60

	// A repost is a public endorsement, a favorite is private. Weight the
	// stronger signal higher.
	suggestedFollowsFavoriteWeight = 1.0
	suggestedFollowsRepostWeight   = 1.5
)

/*
Suggests artists to follow based on the user's own favorites and reposts.

This is the "direct owner" pass: artists whose tracks or albums the user has
already favorited or reposted but has not followed. It is deliberately not a
collaborative filter — those candidates are already engaged with, so they need
no graph traversal to justify, and "you saved three of their tracks and never
followed them" is both the cheapest and the most legible suggestion available.

Distinct from /users/:userId/related, which is artist-anchored ("followers of X
also follow Y"). This one is viewer-anchored.

Suggestions exclude anyone the seed user already follows, so the result depends
only on the path userId — not on the caller — and is cached on that alone.
*/
func (app *ApiServer) v1UsersSuggestedFollows(c *fiber.Ctx) error {
	params := GetUsersSuggestedFollowsParams{}
	if err := app.ParseAndValidateQueryParams(c, &params); err != nil {
		return err
	}

	myId := app.getMyId(c)
	userId := app.getUserId(c)

	candidateIds, err := app.getSuggestedFollowIds(
		c.Context(),
		userId,
		params.Limit,
		params.Offset,
	)
	if err != nil {
		return err
	}

	users, err := app.queries.Users(c.Context(), dbv1.GetUsersParams{
		MyID: myId,
		Ids:  candidateIds,
	})
	if err != nil {
		return err
	}

	return v1UsersResponse(c, users)
}

func (app *ApiServer) getSuggestedFollowIds(
	ctx context.Context,
	userId int32,
	limit int,
	offset int,
) ([]int32, error) {
	cacheKey := fmt.Sprintf("suggested_follows:%d:%d:%d", userId, limit, offset)
	if hit, ok := app.suggestedFollowsCache.Get(cacheKey); ok {
		return hit, nil
	}

	sql := `
	WITH recent_favorites AS (
		SELECT
			save_item_id AS item_id,
			save_type::text AS item_type,
			@favoriteWeight::float8 AS weight,
			created_at
		FROM saves
		WHERE user_id = @userId
			AND is_current = true
			AND is_delete = false
		ORDER BY created_at DESC
		LIMIT @engagementCap
	),
	recent_reposts AS (
		SELECT
			repost_item_id AS item_id,
			repost_type::text AS item_type,
			@repostWeight::float8 AS weight,
			created_at
		FROM reposts
		WHERE user_id = @userId
			AND is_current = true
			AND is_delete = false
		ORDER BY created_at DESC
		LIMIT @engagementCap
	),
	my_engagement AS (
		SELECT * FROM recent_favorites
		UNION ALL
		SELECT * FROM recent_reposts
	),
	-- Attribute each favorite/repost to the artist who owns the item.
	seeds AS (
		SELECT t.owner_id AS user_id, e.weight, e.created_at
		FROM my_engagement e
		JOIN tracks t ON t.track_id = e.item_id
		WHERE e.item_type = 'track'
			AND t.is_current = true
			AND t.is_delete = false
			AND t.is_unlisted = false
			AND t.is_available = true
			AND t.stem_of IS NULL
		UNION ALL
		SELECT p.playlist_owner_id AS user_id, e.weight, e.created_at
		FROM my_engagement e
		JOIN playlists p ON p.playlist_id = e.item_id
		WHERE e.item_type IN ('album', 'playlist')
			AND p.is_current = true
			AND p.is_delete = false
			AND p.is_private = false
	),
	scored AS (
		SELECT
			s.user_id,
			SUM(
				s.weight * exp(
					-EXTRACT(epoch FROM ((now() AT TIME ZONE 'utc') - s.created_at))::float8
						/ @decaySeconds::float8
				)
			) AS score
		FROM seeds s
		GROUP BY s.user_id
	)
	SELECT sc.user_id
	FROM scored sc
	JOIN users u ON u.user_id = sc.user_id
	WHERE sc.user_id != @userId
		AND u.is_current = true
		AND u.is_deactivated = false
		AND u.is_available = true
		AND NOT EXISTS (
			SELECT 1
			FROM follows f
			WHERE f.follower_user_id = @userId
				AND f.followee_user_id = sc.user_id
				AND f.is_current = true
				AND f.is_delete = false
		)
	-- user_id breaks score ties so pagination is stable across pages.
	ORDER BY sc.score DESC, sc.user_id ASC
	LIMIT @limit
	OFFSET @offset
	`

	rows, err := app.pool.Query(ctx, sql, pgx.NamedArgs{
		"userId":         userId,
		"limit":          limit,
		"offset":         offset,
		"engagementCap":  suggestedFollowsEngagementCap,
		"decaySeconds":   suggestedFollowsDecaySeconds,
		"favoriteWeight": suggestedFollowsFavoriteWeight,
		"repostWeight":   suggestedFollowsRepostWeight,
	})
	if err != nil {
		return nil, err
	}
	ids, err := pgx.CollectRows(rows, pgx.RowTo[int32])
	if err != nil {
		return nil, err
	}

	app.suggestedFollowsCache.Set(cacheKey, ids)
	return ids, nil
}
