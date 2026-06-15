package api

import (
	"github.com/gofiber/fiber/v2"
	"go.uber.org/zap"
)

func (app *ApiServer) postV1UsersPing(c *fiber.Ctx) error {
	if app.writePool == nil {
		return fiber.NewError(fiber.StatusServiceUnavailable, "writes not available")
	}

	wallet := app.getAuthedWallet(c)

	_, err := app.writePool.Exec(c.Context(), `
		UPDATE users
		SET last_active_at = now()
		WHERE wallet = $1
		  AND is_current = true
	`, wallet)
	if err != nil {
		app.logger.Error("postV1UsersPing: failed to update last_active_at", zap.Error(err))
		return fiber.NewError(fiber.StatusInternalServerError, "failed to record activity")
	}

	return c.JSON(fiber.Map{"status": "ok"})
}
