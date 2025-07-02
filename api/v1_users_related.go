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
		// Simple approach: find who recent followers also follow
		sql = `
		WITH recent_followers AS MATERIALIZED (
			SELECT follower_user_id
			FROM follows 
			WHERE followee_user_id = @userId
			ORDER BY follower_user_id DESC
			LIMIT 500
		)
		SELECT 
			f.followee_user_id AS user_id
		FROM recent_followers rf
		JOIN LATERAL (
			SELECT followee_user_id
			FROM follows f
			WHERE f.follower_user_id = rf.follower_user_id
				AND f.followee_user_id != @userId
			ORDER BY followee_user_id
			LIMIT 200
		) f ON true
		JOIN users u ON u.user_id = f.followee_user_id
		JOIN aggregate_user au ON au.user_id = f.followee_user_id
		WHERE u.is_current = true
			AND u.is_deactivated = false
			AND u.is_available = true
			AND au.follower_count > 10
		GROUP BY f.followee_user_id, au.follower_count
		HAVING COUNT(*) >= 3
		ORDER BY COUNT(*)::float / (500 + au.follower_count - COUNT(*)) DESC
		LIMIT @limit
		OFFSET @offset
		`
	}

	// WITH recent_followers AS (
	// 	SELECT follower_user_id
	// 	FROM follows
	// 	WHERE followee_user_id = @userId
	// 	AND is_delete = false
	// 	LIMIT 200  -- Just use first 200 followers (by insert order)
	// )
	// SELECT f.followee_user_id as user_id
	// FROM recent_followers rf
	// JOIN follows f ON f.follower_user_id = rf.follower_user_id
	// JOIN users u ON u.user_id = f.followee_user_id
	// JOIN aggregate_user au ON au.user_id = f.followee_user_id
	// WHERE f.is_delete = false
	// AND f.followee_user_id != @userId
	// AND u.is_deactivated = false
	// AND u.is_available = true
	// AND au.follower_count > 10
	// AND (
	// 	@filterFollowed = false
	// 	OR @myId = 0
	// 	OR NOT EXISTS(
	// 		SELECT 1
	// 		FROM follows AS follow_check
	// 		WHERE follow_check.is_delete = false
	// 		AND follow_check.follower_user_id = @myId
	// 		AND follow_check.followee_user_id = f.followee_user_id
	// 	)
	// )
	// GROUP BY f.followee_user_id, au.follower_count
	// ORDER BY COUNT(*) DESC, au.follower_count DESC
	// LIMIT @limit
	// OFFSET @offset

	filterFollowed, _ := strconv.ParseBool(c.Query("filter_followed"))
	return app.queryFullUsers(c, sql, pgx.NamedArgs{
		"myId":           app.getMyId(c),
		"userId":         app.getUserId(c),
		"filterFollowed": filterFollowed,
	})
}
