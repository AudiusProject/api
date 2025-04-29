package api

import (
	"bridgerton.audius.co/api/dbv1"
	"github.com/gofiber/fiber/v2"
)

func (app *ApiServer) v1User(c *fiber.Ctx) error {
	myId := app.getMyId(c)
	userId := c.Locals("userId").(int)

	users, err := app.queries.FullUsers(c.Context(), dbv1.GetUsersParams{
		MyID: myId,
		Ids:  []int32{int32(userId)},
	})

	if err != nil {
		return err
	}

	if len(users) == 0 {
		return sendError(c, 404, "user not found")
	}

	// full returns an array
	// non-full returns an object
	// wild
	if app.getIsFull(c) {
		return v1UsersResponse(c, users)
	}
	return v1UserResponse(c, users[0])
}
