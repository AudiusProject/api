package api

import (
	"bridgerton.audius.co/trashid"
	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"
)

func (app *ApiServer) v1Comment(c *fiber.Ctx) error {
	sql := `
	SELECT comment_id
	FROM comments
	WHERE comment_id = @comment_id
	AND is_delete = false
	`

	commentId, err := trashid.DecodeHashId(c.Params("commentId"))
	if err != nil {
		return err
	}

	return app.queryFullComments(c, sql, pgx.NamedArgs{
		"comment_id": commentId,
	})
}
