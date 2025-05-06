package api

import (
	"strconv"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5/pgtype"
)

func getOptionalBool(c *fiber.Ctx, key string) (pgtype.Bool, error) {
	if valueStr := c.Query(key); valueStr != "" {
		parsed, err := strconv.ParseBool(c.Query(key))
		if err != nil {
			return pgtype.Bool{}, err
		}
		return pgtype.Bool{Bool: parsed, Valid: true}, nil
	}
	return pgtype.Bool{}, nil
}
