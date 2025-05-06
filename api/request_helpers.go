package api

import (
	"strconv"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5/pgtype"
)

func getOptionalBool(c *fiber.Ctx, key string) (pgtype.Bool, error) {
	var value *bool
	if valueStr := c.Query(key); valueStr != "" {
		parsed, err := strconv.ParseBool(valueStr)
		if err != nil {
			return pgtype.Bool{}, err
		}
		value = &parsed
	}
	return pgtype.Bool{Bool: value != nil && *value, Valid: value != nil}, nil
}
