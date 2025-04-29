package api

import (
	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"
)

func (app *ApiServer) v1TracksComments(c *fiber.Ctx) error {

	sql := `
	SELECT comment_id as id
	FROM comments
	LEFT JOIN comment_threads USING (comment_id)
	WHERE entity_id = @track_id
	AND parent_comment_id IS NULL
	AND entity_type = 'Track'
	AND comments.is_delete = false
	`

	args := pgx.NamedArgs{
		"track_id": c.Locals("trackId"),
	}

	return app.queryFullComments(c, sql, args)
}
