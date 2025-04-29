package api

import (
	"bridgerton.audius.co/api/dbv1"
	"github.com/gofiber/fiber/v2"
)

func (app *ApiServer) v1UsersHandle(c *fiber.Ctx) error {
	handle := c.Params("handle")
	if handle == "" {
		return fiber.NewError(fiber.StatusBadRequest, "Missing handle parameter")
	}

	userId, err := app.queries.GetUserForHandle(c.Context(), handle)
	if err != nil {
		return err
	}

	users, err := app.queries.FullUsers(c.Context(), dbv1.GetUsersParams{
		Ids:  []int32{int32(userId)},
		MyID: userId,
	})
	if err != nil {
		return err
	}

	return c.JSON(fiber.Map{
		"data": users[0],
	})
}
