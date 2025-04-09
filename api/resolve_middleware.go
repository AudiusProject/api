package api

import (
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
	if err != nil || userId == 0 {
		return sendError(c, 400, "invalid userId")
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
