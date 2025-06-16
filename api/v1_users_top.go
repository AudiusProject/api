package api

import (
	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"
)

func (app *ApiServer) v1UsersTop(c *fiber.Ctx) error {
	sql := `
		SELECT users.user_id
		FROM users
		JOIN aggregate_user using (user_id)
		WHERE
			is_current = TRUE
			AND track_count > 0
		ORDER BY follower_count DESC, user_id ASC
		LIMIT @limit
		OFFSET @offset
	;`

	return app.queryFullUsers(c, sql, pgx.NamedArgs{})
}
