package api

import (
	"net/http"

	"bridgerton.audius.co/trashid"
	"github.com/gofiber/fiber/v2"
)

// will set myId if valid, defaults to 0
func (app *ApiServer) resolveMyIdMiddleware(c *fiber.Ctx) error {
	myId, _ := trashid.DecodeHashId(c.Query("user_id"))
	c.Locals("myId", myId)
	return c.Next()
}

func (app *ApiServer) requireUserIdMiddleware(c *fiber.Ctx) error {
	userId, err := trashid.DecodeHashId(c.Params("userId"))
	if err != nil {
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{
			"code":  http.StatusBadRequest,
			"error": "Invalid userId",
		})
	}
	c.Locals("userId", userId)
	return c.Next()
}

func (app *ApiServer) requireHandleMiddleware(c *fiber.Ctx) error {
	userId, err := app.resolveUserHandleToId(c.Params("handle"))
	if err != nil {
		return err
	}
	c.Locals("userId", userId)
	return c.Next()
}
