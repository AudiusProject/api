package api

import (
	"bridgerton.audius.co/api/dbv1"
	"github.com/gofiber/fiber/v2"
)

func (app *ApiServer) v1playlists(c *fiber.Ctx) error {
	myId := app.getMyId(c)
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
