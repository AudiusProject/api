package api

import (
	"api.audius.co/api/dbv1"
	"github.com/gofiber/fiber/v2"
)

type GetPlaylistsSearchParams struct {
	Type string `query:"type" default:"playlist" validate:"oneof=playlist album"`
}

func (app *ApiServer) v1PlaylistsSearch(c *fiber.Ctx) error {
	params := GetPlaylistsSearchParams{}
	if err := app.ParseAndValidateQueryParams(c, &params); err != nil {
		return err
	}

	var playlists []dbv1.Playlist
	var err error
	if params.Type == "album" {
		playlists, err = app.searchAlbums(c)
	} else {
		playlists, err = app.searchPlaylists(c)
	}
	if err != nil {
		return err
	}

	return c.JSON(fiber.Map{
		"data": playlists,
	})
}
