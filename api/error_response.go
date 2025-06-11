package api

import (
	"errors"
	"net/http"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"
	"go.uber.org/zap"
)

func errorHandler(logger *zap.Logger) func(*fiber.Ctx, error) error {
	return func(ctx *fiber.Ctx, err error) error {
		code := http.StatusInternalServerError
		if err == pgx.ErrNoRows {
			code = http.StatusNotFound
		}

		var e *fiber.Error
		if errors.As(err, &e) {
			code = e.Code
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
