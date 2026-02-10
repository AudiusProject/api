package api

import (
	"api.audius.co/trashid"
	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"
)

func (app *ApiServer) v1PlaylistReposts(c *fiber.Ctx) error {
	sql := `
	SELECT user_id
	FROM reposts r
	JOIN users u using (user_id)
	JOIN aggregate_user au using (user_id)
	WHERE repost_type != 'track'
	  AND repost_item_id = @playlistId
	  AND is_delete = false
	  AND u.is_deactivated = false
	ORDER BY follower_count desc
	LIMIT @limit
	OFFSET @offset
	`

	playlistId, err := trashid.DecodeHashId(c.Params("playlistId"))
	if err != nil {
		return err
	}

	return app.queryUsers(c, sql, pgx.NamedArgs{
		"playlistId": playlistId,
	})
}
