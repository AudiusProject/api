package api

import (
	"strings"

	"bridgerton.audius.co/trashid"
	"github.com/gofiber/fiber/v2"
)

func (app *ApiServer) isFullMiddleware(c *fiber.Ctx) error {
	u := c.OriginalURL()
	isFull := strings.Contains(u, "/full/")
	c.Locals("isFull", isFull)
	return c.Next()
}

func (app *ApiServer) getIsFull(c *fiber.Ctx) bool {
	return c.Locals("isFull").(bool)
}

// will set myId if valid, defaults to 0
func (app *ApiServer) resolveMyIdMiddleware(c *fiber.Ctx) error {
	myId, _ := trashid.DecodeHashId(c.Query("user_id"))
	c.Locals("myId", myId)
	return c.Next()
}

func (app *ApiServer) getMyId(c *fiber.Ctx) int32 {
	myId := c.Locals("myId")
	if myId == nil {
		return 0
	}
	return int32(myId.(int))
}

// Gets the decoded user ID from the path parameter
func (app *ApiServer) getUserId(c *fiber.Ctx) int32 {
	userId := c.Locals("userId")
	if userId == nil {
		return 0
	}
	return int32(userId.(int))
}

func (app *ApiServer) requireUserIdMiddleware(c *fiber.Ctx) error {
	userId, err := trashid.DecodeHashId(c.Params("userId"))
	if err != nil || userId == 0 {
		return fiber.NewError(fiber.StatusBadRequest, "invalid userId")
	}
	c.Locals("userId", userId)
	return c.Next()
}

func (app *ApiServer) requireHandleMiddleware(c *fiber.Ctx) error {
	userId, err := app.resolveUserHandleToId(c.Params("handle"))
	if err != nil {
		return err
	}
	c.Locals("userId", int(userId))
	return c.Next()
}

func (app *ApiServer) requireTrackIdMiddleware(c *fiber.Ctx) error {
	trackId, err := trashid.DecodeHashId(c.Params("trackId"))
	if err != nil || trackId == 0 {
		return fiber.NewError(fiber.StatusBadRequest, "invalid trackId")
	}
	c.Locals("trackId", trackId)
	return c.Next()
}

func (app *ApiServer) requirePlaylistIdMiddleware(c *fiber.Ctx) error {
	playlistId, err := trashid.DecodeHashId(c.Params("playlistId"))
	if err != nil || playlistId == 0 {
		return fiber.NewError(fiber.StatusBadRequest, "invalid playlistId")
	}
	c.Locals("playlistId", playlistId)
	return c.Next()
}
