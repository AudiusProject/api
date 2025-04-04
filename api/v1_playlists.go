package api

import (
	"bridgerton.audius.co/api/dbv1"
	"bridgerton.audius.co/trashid"
	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"
)

func (app *ApiServer) v1playlists(c *fiber.Ctx, minResponse bool) error {
	myId, _ := trashid.DecodeHashId(c.Query("user_id"))
	ids := decodeIdList(c)

	playlists, err := app.queries.FullPlaylists(c.Context(), dbv1.GetPlaylistsParams{
		MyID: myId,
		Ids:  ids,
	})
	if err != nil {
		return err
	}

	if minResponse {
		return c.JSON(fiber.Map{
			"data": dbv1.ToMinPlaylists(playlists),
		})
	}

	return c.JSON(fiber.Map{
		"data": playlists,
	})
}

func (app *ApiServer) v1PlaylistsReposts(c *fiber.Ctx, minResponse bool) error {
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

	return app.queryFullUsers(c, sql, pgx.NamedArgs{
		"playlistId": playlistId,
	}, minResponse)
}

func (app *ApiServer) v1PlaylistsFavorites(c *fiber.Ctx, minResponse bool) error {
	sql := `
	SELECT user_id
	FROM saves
	JOIN users u using (user_id)
	JOIN aggregate_user au using (user_id)
	WHERE save_type != 'track'
	  AND save_item_id = @playlistId
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

	return app.queryFullUsers(c, sql, pgx.NamedArgs{
		"playlistId": playlistId,
	}, minResponse)
}
