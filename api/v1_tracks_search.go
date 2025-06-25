package api

import (
	"github.com/gofiber/fiber/v2"
)

func (app *ApiServer) v1TracksSearch(c *fiber.Ctx) error {
	tracks, err := app.searchTracks(c)
	if err != nil {
		return err
	}

	return c.JSON(fiber.Map{
		"data": tracks,
	})
}
