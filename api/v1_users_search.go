package api

import (
	"github.com/gofiber/fiber/v2"
)

func (app *ApiServer) v1UsersSearch(c *fiber.Ctx) error {
	users, err := app.searchUsers(c)
	if err != nil {
		return err
	}

	return c.JSON(fiber.Map{
		"data": users,
	})
}
