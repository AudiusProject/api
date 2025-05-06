package api

import (
	"github.com/gofiber/fiber/v2"
)

func (app *ApiServer) v1UsersTransactionsUsdc(c *fiber.Ctx) error {
	return c.JSON(fiber.Map{
		"data": []fiber.Map{},
	})
}

func (app *ApiServer) v1UsersTransactionsUsdcCount(c *fiber.Ctx) error {
	return c.JSON(fiber.Map{
		"data": 0,
	})
}
