package api

import (
	"github.com/gofiber/fiber/v2"
)

func (app *ApiServer) v1UsersConnectedWallets(c *fiber.Ctx) error {
	userId := c.Locals("userId").(int)

	wallets, err := app.queries.FullConnectedWallets(c.Context(), int32(userId))
	if err != nil {
		return err
	}

	return c.JSON(fiber.Map{
		"data": wallets,
	})
}
