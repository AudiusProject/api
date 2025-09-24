package api

import (
	"github.com/gofiber/fiber/v2"
)

func (app *ApiServer) v1MetricsTotalWallets(c *fiber.Ctx) error {
	var total int64
	if err := app.pool.QueryRow(c.Context(), `
		SELECT SUM(count)::bigint AS total
		FROM (
			SELECT COUNT(*)::bigint AS count FROM users
			UNION ALL
			SELECT COUNT(*)::bigint AS count FROM sol_claimable_accounts
		) AS combined
    `).Scan(&total); err != nil {
		return err
	}

	return c.JSON(fiber.Map{"data": map[string]int64{"total": total}})
}
