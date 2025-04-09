package api

import (
	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"
)

func (app *ApiServer) v1UsersFollowing(c *fiber.Ctx) error {

	sql := `
	SELECT followee_user_id
	FROM follows
	JOIN users on followee_user_id = users.user_id
	JOIN aggregate_user using (user_id)
	WHERE follower_user_id = @userId
	  AND is_delete = false
	  AND is_deactivated = false
	ORDER BY follower_count desc
	LIMIT @limit
	OFFSET @offset
	`

	userId := c.Locals("userId").(int)
	return app.queryFullUsers(c, sql, pgx.NamedArgs{
		"userId": userId,
	})
}
