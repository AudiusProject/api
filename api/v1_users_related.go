package api

import (
	"strconv"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"
)

/*
Hybrid approach:
- For artists with < 100 followers: genre-based recommendations (not enough follower data)
- For artists with >= 100 followers: collaborative filtering with small genre boost
*/
func (app *ApiServer) v1UsersRelated(c *fiber.Ctx) error {
	var followerCount int64
	err := app.pool.QueryRow(
		c.Context(),
		`SELECT follower_count FROM aggregate_user WHERE user_id = $1`,
		app.getUserId(c),
	).Scan(&followerCount)
	if err != nil {
		return err
	}

	var sql string

	// Use different algorithms based on follower count
	if followerCount < 100 {
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
		OFFSET @offset
		`
	} else {
		// Simple approach: find who recent followers also follow
		sql = `
		WITH recent_followers AS MATERIALIZED (
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
			FROM recent_followers rf
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
		)
		SELECT user_id
		FROM scored_candidates
		WHERE shared_followers >= 3
		ORDER BY 
			-- Approx. jaccard similarity with small genre boost
			(shared_followers::float / (500 + follower_count - shared_followers)) + (genre_match * 0.05) DESC
		LIMIT @limit
		OFFSET @offset;
		`
	}

	filterFollowed, _ := strconv.ParseBool(c.Query("filter_followed"))
	return app.queryFullUsers(c, sql, pgx.NamedArgs{
		"myId":           app.getMyId(c),
		"userId":         app.getUserId(c),
		"filterFollowed": filterFollowed,
	})
}
