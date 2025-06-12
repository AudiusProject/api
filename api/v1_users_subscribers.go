package api

import (
	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"
)

func (app *ApiServer) v1UsersSubscribers(c *fiber.Ctx) error {
	sql := `
		SELECT
			subscriber_id
		FROM
			subscriptions
		WHERE
			user_id = @userId
			AND is_current = true
			AND is_delete = false
		ORDER BY
			subscriber_id asc
		OFFSET @offset
		LIMIT @limit
	;`

	return app.queryFullUsers(c, sql, pgx.NamedArgs{
		"userId": app.getUserId(c),
	})
}
