package api

import (
	"github.com/gofiber/fiber/v2"
)

func (app *ApiServer) v1UsersTransactionsAudio(c *fiber.Ctx) error {
	return c.JSON(fiber.Map{
		"data": []fiber.Map{},
	})
}

func (app *ApiServer) v1UsersTransactionsAudioCount(c *fiber.Ctx) error {
	return c.JSON(fiber.Map{
		"data": 0,
	})
}
