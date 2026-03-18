package api

import (
	"github.com/gofiber/fiber/v2"
)

// v1UserTracksDownloadCount returns the total download count for all tracks
// (and stems) owned by the user. Use this for dashboard "Downloads" tile
// instead of summing per-track counts client-side.
//
// GET /v1/full/users/:userId/tracks/download_count
func (app *ApiServer) v1UserTracksDownloadCount(c *fiber.Ctx) error {
	userId := app.getUserId(c)

	total, err := app.queries.GetUserTrackDownloadCountTotal(c.Context(), userId)
	if err != nil {
		return err
	}

	return c.JSON(fiber.Map{
		"data": total,
	})
}
