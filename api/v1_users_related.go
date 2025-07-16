package api

import (
	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"
)

type GetUsersRelatedParams struct {
	Limit          int  `query:"limit" default:"10" validate:"min=1,max=100"`
	Offset         int  `query:"offset" default:"0" validate:"min=0"`
	FilterFollowed bool `query:"filter_followed" default:"false" validate:"boolean"`
}

/*
Hybrid approach:
- For artists with < 100 followers: genre-based recommendations (not enough follower data)
- For artists with >= 100 followers: collaborative filtering with small genre boost
*/
func (app *ApiServer) v1UsersRelated(c *fiber.Ctx) error {
	params := GetUsersRelatedParams{}
	if err := app.ParseAndValidateQueryParams(c, &params); err != nil {
		return err
	}

	var followerCount int64
	err := app.pool.QueryRow(
		c.Context(),
		`SELECT follower_count FROM aggregate_user WHERE user_id = $1`,
		app.getUserId(c),
	).Scan(&followerCount)
	if err != nil {
		return err
	}
	lowFollowerCount := followerCount < 100

	var sql string

	// Use different algorithms based on follower count
	if lowFollowerCount {
		// Genre-based algorithm for smaller artists
		sql = `
		WITH inp AS (
			SELECT genre,
				count(*) AS track_count,
				rank() OVER (ORDER BY count(*) DESC) AS genre_rank
			FROM tracks AS t
			WHERE t.is_current IS true
				AND t.is_delete IS false
				AND t.is_unlisted IS false
				AND t.is_available IS true
				AND t.stem_of IS NULL
				AND owner_id = @userId
			GROUP BY genre
			ORDER BY count(*) DESC
			LIMIT 5
		)
		SELECT user_id
		FROM aggregate_user AS au
		JOIN users USING (user_id)
		JOIN inp ON dominant_genre = inp.genre
		WHERE user_id != @userId
		AND is_deactivated = false
		AND is_available = true
		AND au.follower_count < (SELECT follower_count * 3 FROM aggregate_user WHERE user_id = @userId)
		AND (
			@filterFollowed = false
			OR @myId = 0
			OR NOT EXISTS(
				SELECT 1
				FROM follows AS f
				WHERE f.is_current = true
				AND f.is_delete = false
				AND f.follower_user_id = @myId
				AND f.followee_user_id = au.user_id
			)
		)
		ORDER BY genre_rank ASC, follower_count DESC
		LIMIT @limit
		OFFSET @offset;
		`
	} else {
		// simple collaborative filtering
		// - get a sample of followers. as ids are random, this is a reasonable sample
		// - for each follower, get the top 200 artists they follow
		// - score candidates based on how many of our sample follow them with some genre boost
		// - return the top n
		sql = `
		WITH followers_sample AS MATERIALIZED (
			SELECT follower_user_id
			FROM follows 
			WHERE followee_user_id = @userId
			ORDER BY follower_user_id DESC
			LIMIT 500
		),
		top_genres AS (
			SELECT genre
			FROM tracks
			WHERE owner_id = @userId
				AND is_current = true
				AND is_delete = false
				AND is_unlisted = false
				AND is_available = true
				AND stem_of IS NULL
				AND genre IS NOT NULL
			GROUP BY genre
			ORDER BY COUNT(*) DESC
			LIMIT 3
		),
		candidate_users AS (
			SELECT 
				f.followee_user_id AS user_id,
				COUNT(*) AS shared_followers
			FROM followers_sample rf
			JOIN LATERAL (
				SELECT followee_user_id
				FROM follows f
				WHERE f.follower_user_id = rf.follower_user_id
					AND f.followee_user_id != @userId
				ORDER BY followee_user_id DESC
				LIMIT 200
			) f ON true
			GROUP BY f.followee_user_id
		),
		scored_candidates AS (
			SELECT 
				cu.user_id,
				cu.shared_followers,
				au.follower_count,
				CASE 
					WHEN au.dominant_genre IN (SELECT genre FROM top_genres) THEN 1
					ELSE 0
				END AS genre_match
			FROM candidate_users cu
			JOIN users u ON u.user_id = cu.user_id
			JOIN aggregate_user au ON au.user_id = cu.user_id
			WHERE u.is_current = true
				AND u.is_deactivated = false
				AND u.is_available = true
				AND au.follower_count > 10
				AND (
					@filterFollowed = false
					OR @myId = 0
					OR NOT EXISTS(
						SELECT 1
						FROM follows AS f
						WHERE f.is_current = true
						AND f.is_delete = false
						AND f.follower_user_id = @myId
						AND f.followee_user_id = au.user_id
					)
				)
		)
		SELECT user_id
		FROM scored_candidates
		WHERE shared_followers >= 3
		ORDER BY 
			-- approx jaccard similarity with small genre boost
			(shared_followers::float / (500 + follower_count - shared_followers)) + (genre_match * 0.05) DESC
		LIMIT @limit
		OFFSET @offset;
		`
	}

	var limit int
	if lowFollowerCount {
		// Clamp results to 0-10 because results are not as
		// good for low follower counts
		limit = min(params.Limit, max(0, 10-params.Offset))
	} else {
		limit = params.Limit
	}

	return app.queryFullUsers(c, sql, pgx.NamedArgs{
		"myId":           app.getMyId(c),
		"userId":         app.getUserId(c),
		"filterFollowed": params.FilterFollowed,
		"limit":          limit,
		"offset":         params.Offset,
	})
}
