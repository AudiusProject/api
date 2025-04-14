package api

import (
	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"
)

func (app *ApiServer) v1UsersMutuals(c *fiber.Ctx) error {

	sql := `
	SELECT x.follower_user_id
	FROM follows x
	JOIN aggregate_user au on x.follower_user_id = au.user_id
	JOIN follows me
	  ON me.follower_user_id = @myId
	 AND me.followee_user_id = x.follower_user_id
	 AND me.is_delete = false
	WHERE x.followee_user_id = @userId
	  AND x.is_delete = false
	ORDER BY follower_count DESC
	LIMIT @limit
	OFFSET @offset
	`

	myId := c.Locals("myId")
	userId := c.Locals("userId").(int)
	return app.queryFullUsers(c, sql, pgx.NamedArgs{
		"myId":   myId,
		"userId": userId,
	})
}
