package api

import (
	"github.com/gofiber/fiber/v2"
	"go.uber.org/zap"
)

func (app *ApiServer) postV1UsersPing(c *fiber.Ctx) error {
	if app.writePool == nil {
		return fiber.NewError(fiber.StatusServiceUnavailable, "writes not available")
	}

	myId := app.getMyId(c)
	if myId == 0 {
		return fiber.NewError(fiber.StatusBadRequest, "user_id query param is required")
	}

	_, err := app.writePool.Exec(c.Context(), `
		UPDATE users
		SET last_active_at = now()
		WHERE user_id = $1
		  AND is_current = true
	`, myId)
	if err != nil {
		app.logger.Error("postV1UsersPing: failed to update last_active_at", zap.Error(err))
		return fiber.NewError(fiber.StatusInternalServerError, "failed to record activity")
	}

	return c.JSON(fiber.Map{"status": "ok"})
}
