package api

import (
	"fmt"
	"strconv"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"
)

/*
/v1/full/users/ePQV7/related?limit=5&offset=0&user_id=aNzoj
*/

func (app *ApiServer) v1UsersRelated(c *fiber.Ctx) error {

	sql := `
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
	AND au.follower_count < (SELECT follower_count * 3 FROM aggregate_user WHERE user_id = @userId)
	WHERE user_id != @userId
	AND is_deactivated = false
	AND is_available = true
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

	filterFollowed, _ := strconv.ParseBool(c.Query("filter_followed"))
	fmt.Println("_________________ FILT FOLL", filterFollowed)
	return app.queryFullUsers(c, sql, pgx.NamedArgs{
		"myId":           c.Locals("myId"),
		"userId":         c.Locals("userId"),
		"filterFollowed": filterFollowed,
	})
}
