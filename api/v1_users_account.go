package api

import (
	"encoding/json"

	"bridgerton.audius.co/api/dbv1"
	"github.com/gofiber/fiber/v2"
)

// todo: in python this route requires auth
// todo: fill out additional fields: track_save_count, playlists, playlist_library
func (app *ApiServer) v1UsersAccount(c *fiber.Ctx) error {
	myId := app.getMyId(c)

	// resolve wallet to user id
	var userId int32
	err := app.pool.QueryRow(c.Context(),
		`SELECT user_id FROM users where wallet = lower($1) ORDER BY (handle IS NOT NULL) DESC, created_at ASC`,
		c.Params("wallet"),
	).Scan(&userId)

	if err != nil {
		return err
	}

	users, err := app.queries.FullUsers(c.Context(), dbv1.GetUsersParams{
		Ids:  []int32{userId},
		MyID: myId,
	})
	if err != nil {
		return err
	}
	if len(users) == 0 {
		return sendError(c, 404, "wallet not found")
	}

	// this route does not have a non-full version...
	// and also nests user under data.user
	// and there are some additional fields
	// so we don't use the v1UserResponse helper
	todoEmptyArray := json.RawMessage([]byte(`[]`))
	return c.JSON(fiber.Map{
		"data": fiber.Map{
			"track_save_count": 0,              // todo
			"playlists":        todoEmptyArray, // todo
			"playlist_library": fiber.Map{
				"contents": todoEmptyArray,
			},
			"user": users[0],
		},
	})
}
