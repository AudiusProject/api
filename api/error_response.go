package api

import (
	"net/http"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"
	"go.uber.org/zap"
)

func sendError(c *fiber.Ctx, status int, message string) error {
	return c.Status(status).JSON(fiber.Map{
		"code":  status,
		"error": message,
	})
}

func errorHandler(logger *zap.Logger) func(*fiber.Ctx, error) error {
	return func(ctx *fiber.Ctx, err error) error {
		code := http.StatusInternalServerError
		if err == pgx.ErrNoRows {
			code = http.StatusNotFound
		}

		if code > 499 {
			logger.Error(err.Error(),
				zap.String("url", ctx.OriginalURL()))
		}

		return ctx.Status(code).JSON(&fiber.Map{
			"code":  code,
			"error": err.Error(),
		})
	}
}
