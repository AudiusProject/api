package api

import (
	"github.com/gofiber/fiber/v2"
)

func (app *ApiServer) v1MetricsTotalPlays(c *fiber.Ctx) error {
	var total int64
	if err := app.pool.QueryRow(c.Context(), `
        SELECT COALESCE(SUM(count), 0)::bigint
        FROM aggregate_plays
    `).Scan(&total); err != nil {
		return err
	}

	return c.JSON(fiber.Map{"data": map[string]int64{"total": total}})
}
