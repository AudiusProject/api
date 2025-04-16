package api

import (
	"encoding/json"

	"bridgerton.audius.co/api/dbv1"
	"github.com/gofiber/fiber/v2"
)

// todo: in python this route requires auth
// todo: fill out additional fields: track_save_count, playlists
func (app *ApiServer) v1UsersAccount(c *fiber.Ctx) error {
	// resolve wallet to user id
	var userId int32

	// todo: this is a duplicate of the authMiddleware, make it common?
	err := app.pool.QueryRow(
		c.Context(),
		`
		SELECT user_id FROM users
		WHERE
			wallet = lower($1)
			AND is_current = true
		ORDER BY handle_lc IS NOT NULL, created_at ASC
		LIMIT 1
		`,
		c.Params("wallet"),
	).Scan(&userId)

	if err != nil {
		return err
	}

	users, err := app.queries.FullUsers(c.Context(), dbv1.GetUsersParams{
		Ids:  []int32{userId},
		MyID: userId,
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

	// Extract playlist_library from user record
	playlistLibrary := users[0].PlaylistLibrary
	// Create a copy of the user without playlist_library as it's a
	// deprecated field and we will return it as a sibling
	userWithoutLibrary := users[0]
	userWithoutLibrary.PlaylistLibrary = nil

	trackSaveCount, err := app.queries.GetExtendedAccountFields(c.Context(), userId)
	if err != nil {
		return err
	}

	return c.JSON(fiber.Map{
		"data": fiber.Map{
			"track_save_count": trackSaveCount,
			"playlists":        todoEmptyArray, // todo
			"playlist_library": playlistLibrary,
			"user": userWithoutLibrary,
		},
	})
}
