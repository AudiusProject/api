package api

import (
	"bridgerton.audius.co/trashid"
	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"
)

func (app *ApiServer) v1TracksReposts(c *fiber.Ctx) error {
	sql := `
	SELECT user_id
	FROM reposts r
	JOIN users u using (user_id)
	JOIN aggregate_user au using (user_id)
	WHERE repost_type = 'track'
	  AND repost_item_id = @trackId
	  AND is_delete = false
	  AND u.is_deactivated = false
	ORDER BY follower_count desc
	LIMIT @limit
	OFFSET @offset
	`

	trackId, err := trashid.DecodeHashId(c.Params("trackId"))
	if err != nil {
		return err
	}

	return app.queryFullUsers(c, sql, pgx.NamedArgs{
		"trackId": trackId,
	})
}
