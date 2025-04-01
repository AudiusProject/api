package api

import (
	"bridgerton.audius.co/queries"
	"bridgerton.audius.co/trashid"
	"github.com/gofiber/fiber/v2"
)

func (app *ApiServer) v1playlists(c *fiber.Ctx) error {
	myId, _ := trashid.DecodeHashId(c.Query("user_id"))
	ids := decodeIdList(c)

	playlists, err := app.queries.FullPlaylists(c.Context(), queries.GetPlaylistsParams{
		MyID: myId,
		Ids:  ids,
	})
	if err != nil {
		return err
	}

	return c.JSON(fiber.Map{
		"data": playlists,
	})
}
