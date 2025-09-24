package api

import (
	"github.com/gofiber/fiber/v2"
)

func (app *ApiServer) v1MetricsTotalArtists(c *fiber.Ctx) error {
	var total int64
	if err := app.pool.QueryRow(c.Context(), `
        SELECT COUNT(*)::bigint
        FROM aggregate_user
        WHERE total_track_count > 0
    `).Scan(&total); err != nil {
		return err
	}

	return c.JSON(fiber.Map{"data": map[string]int64{"total": total}})
}
