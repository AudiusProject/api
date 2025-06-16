package api

import (
	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"
)

func (app *ApiServer) v1UsersMuted(c *fiber.Ctx) error {
	sql := `
		SELECT
			muted_user_id
		FROM
			muted_users
		WHERE
			user_id = @userId
			AND is_delete = FALSE
	;`

	return app.queryFullUsers(c, sql, pgx.NamedArgs{
		"userId": app.getUserId(c),
	})
}
