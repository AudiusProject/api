package api

import (
	"bridgerton.audius.co/queries"
	"bridgerton.audius.co/trashid"
	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"
)

func (app *ApiServer) v1Tracks(c *fiber.Ctx) error {
	myId, _ := trashid.DecodeHashId(c.Query("user_id"))
	ids := decodeIdList(c)

	tracks, err := app.queries.FullTracks(c.Context(), queries.GetTracksParams{
		MyID: int32(myId),
		Ids:  ids,
	})
	if err != nil {
		return err
	}
	return c.JSON(fiber.Map{
		"data": tracks,
	})
}

func (app *ApiServer) v1TrackReposts(c *fiber.Ctx) error {
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

	users, err := app.queryFullUsers(c, sql, pgx.NamedArgs{
		"trackId": trackId,
	})
	if err != nil {
		return err
	}

	return c.JSON(fiber.Map{
		"data": users,
	})
}

func (app *ApiServer) v1TrackFavorites(c *fiber.Ctx) error {
	sql := `
	SELECT user_id
	FROM saves
	JOIN users u using (user_id)
	JOIN aggregate_user au using (user_id)
	WHERE save_type = 'track'
	  AND save_item_id = @trackId
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

	users, err := app.queryFullUsers(c, sql, pgx.NamedArgs{
		"trackId": trackId,
	})
	if err != nil {
		return err
	}

	return c.JSON(fiber.Map{
		"data": users,
	})
}
