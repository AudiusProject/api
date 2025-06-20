package api

import (
	"github.com/gofiber/fiber/v2"
)

func (app *ApiServer) v1PlaylistsSearch(c *fiber.Ctx) error {
	playlists, err := app.searchPlaylists(c)
	if err != nil {
		return err
	}

	return c.JSON(fiber.Map{
		"data": playlists,
	})
}
