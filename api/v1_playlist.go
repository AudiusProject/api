package api

import (
	"bridgerton.audius.co/api/dbv1"
	"github.com/gofiber/fiber/v2"
)

func (app *ApiServer) v1Playlist(c *fiber.Ctx) error {
	myId := app.getMyId(c)
	playlistId := c.Locals("playlistId").(int)

	playlists, err := app.queries.FullPlaylists(c.Context(), dbv1.FullPlaylistsParams{
		GetPlaylistsParams: dbv1.GetPlaylistsParams{
			MyID: myId,
			Ids:  []int32{int32(playlistId)},
		},
	})
	if err != nil {
		return err
	}

	if len(playlists) == 0 {
		return sendError(c, 404, "playlist not found")
	}

	playlist := playlists[0]

	return v1PlaylistResponse(c, playlist)
}
