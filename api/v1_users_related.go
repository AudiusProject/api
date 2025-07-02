package api

import (
	"fmt"
	"strconv"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"
)

/*
/v1/full/users/ePQV7/related?limit=5&offset=0&user_id=aNzoj

This endpoint uses a hybrid approach:
- For artists with < 200 followers: genre-based recommendations (not enough follower data)
- For artists with >= 200 followers: collaborative filtering based on follower overlap
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
	fmt.Println("followerCount", followerCount)

	var sql string

	// Use different algorithms based on follower count
	if followerCount < 200 {
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
		// Collaborative filtering algorithm for larger artists
		sql = `
		WITH target_followers AS (
			-- Get all followers of the target artist
			SELECT follower_user_id
			FROM follows
			WHERE followee_user_id = @userId
			AND is_current = true
			AND is_delete = false
		),
		candidate_artists AS (
			-- Find artists followed by the target artist's followers
			SELECT 
				f.followee_user_id as candidate_user_id,
				COUNT(*) as shared_followers
			FROM follows f
			INNER JOIN target_followers tf ON f.follower_user_id = tf.follower_user_id
			WHERE f.is_current = true
			AND f.is_delete = false
			AND f.followee_user_id != @userId  -- Don't include the target artist
			GROUP BY f.followee_user_id
			HAVING COUNT(*) >= 2  -- Require at least 2 shared followers
		),
		similarity_scores AS (
			-- Calculate Jaccard similarity: |A ∩ B| / |A ∪ B|
			SELECT 
				ca.candidate_user_id,
				ca.shared_followers,
				target_au.follower_count as target_follower_count,
				candidate_au.follower_count as candidate_follower_count,
				-- Jaccard similarity = shared / (target + candidate - shared)
				CASE 
					WHEN (target_au.follower_count + candidate_au.follower_count - ca.shared_followers) > 0
					THEN ca.shared_followers::float / (target_au.follower_count + candidate_au.follower_count - ca.shared_followers)
					ELSE 0
				END as jaccard_similarity
			FROM candidate_artists ca
			JOIN aggregate_user target_au ON target_au.user_id = @userId
			JOIN aggregate_user candidate_au ON candidate_au.user_id = ca.candidate_user_id
			WHERE candidate_au.follower_count > 0  -- Avoid division by zero
		)
		SELECT ss.candidate_user_id as user_id
		FROM similarity_scores ss
		JOIN users u ON u.user_id = ss.candidate_user_id
		JOIN aggregate_user au ON au.user_id = ss.candidate_user_id
		WHERE u.is_current = true
		AND u.is_deactivated = false
		AND u.is_available = true
		AND (
			@filterFollowed = false
			OR @myId = 0
			OR NOT EXISTS(
				SELECT 1
				FROM follows AS f
				WHERE f.is_current = true
				AND f.is_delete = false
				AND f.follower_user_id = @myId
				AND f.followee_user_id = ss.candidate_user_id
			)
		)
		ORDER BY 
			ss.jaccard_similarity DESC,
			ss.shared_followers DESC,
			au.follower_count DESC
		LIMIT @limit
		OFFSET @offset
		`
	}

	filterFollowed, _ := strconv.ParseBool(c.Query("filter_followed"))
	return app.queryFullUsers(c, sql, pgx.NamedArgs{
		"myId":           app.getMyId(c),
		"userId":         app.getUserId(c),
		"filterFollowed": filterFollowed,
	})
}
