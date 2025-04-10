package api

import (
	"fmt"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"
)

func (app *ApiServer) v1UsersFollowers(c *fiber.Ctx) error {

	sql := `
	SELECT follower_user_id
	FROM follows
	JOIN users on follower_user_id = users.user_id
	JOIN aggregate_user using (user_id)
	WHERE followee_user_id = @userId
	  AND is_delete = false
	  AND is_deactivated = false
	ORDER BY follower_count desc
	LIMIT @limit
	OFFSET @offset
	`

	userId := c.Locals("userId").(int)
	res := app.queryFullUsers(c, sql, pgx.NamedArgs{
		"userId": userId,
	})
	fmt.Println(res)
	return res
}
