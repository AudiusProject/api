package api

import "github.com/gofiber/fiber/v2"

// User challenges are retired. Preserve the response shape for older clients.
// Weekly trending awards remain available through the restricted batch API.
func (app *ApiServer) v1UsersChallenges(c *fiber.Ctx) error {
	return c.JSON(fiber.Map{"data": []any{}})
}
