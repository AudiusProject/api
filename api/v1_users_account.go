package api

import (
	"github.com/gofiber/fiber/v2"
)

func (app *ApiServer) v1UsersAccount(c *fiber.Ctx) error {
	wallet := c.Params("wallet")
	if wallet == "" {
		return fiber.NewError(fiber.StatusBadRequest, "Missing wallet parameter")
	}

	account, err := app.queries.FullAccount(c.Context(), wallet)
	if err != nil {
		return err
	}

	return c.JSON(fiber.Map{
		"data": account,
	})
}
