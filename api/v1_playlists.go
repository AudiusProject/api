package api

import (
	"bridgerton.audius.co/api/dbv1"
	"bridgerton.audius.co/trashid"
	"github.com/gofiber/fiber/v2"
)

func (app *ApiServer) v1playlists(c *fiber.Ctx) error {
	myId, _ := trashid.DecodeHashId(c.Query("user_id"))
	ids := decodeIdList(c)

	playlists, err := app.queries.FullPlaylists(c.Context(), dbv1.GetPlaylistsParams{
		MyID: myId,
		Ids:  ids,
	})
	if err != nil {
		return err
	}

	return v1PlaylistsResponse(c, playlists)
}
