package api

import (
	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"
)

func (app *ApiServer) v1UsersComments(c *fiber.Ctx) error {

	sql := `
	SELECT comment_id as id
	FROM comments
	LEFT JOIN comment_threads USING (comment_id)
	WHERE user_id = @user_id
	AND entity_type = 'Track'
	AND comments.is_delete = false
	`

	args := pgx.NamedArgs{
		"user_id": c.Locals("userId"),
	}

	return app.queryFullComments(c, sql, args)
}
