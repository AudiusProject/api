package api

import (
	"bridgerton.audius.co/api/dbv1"
	"github.com/gofiber/fiber/v2"
)

// todo: in python this route requires auth
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
		ORDER BY handle IS NOT NULL, created_at ASC
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

	playlists, err := app.queries.FullAccountPlaylists(c.Context(), userId)
	if err != nil {
		return err
	}

	// Extract playlist_library from user record
	playlistLibrary := users[0].PlaylistLibrary
	trackSaveCount := users[0].TrackSaveCount
	// Create a copy of the user without playlist_library/track_save_count as
	// they are deprecated fields and we will return them as siblings
	userWithoutLibrary := users[0]
	userWithoutLibrary.PlaylistLibrary = nil
	userWithoutLibrary.TrackSaveCount = nil

	// this route does not have a non-full version...
	// and also nests user under data.user
	// and there are some additional fields
	// so we don't use the v1UserResponse helper
	return c.JSON(fiber.Map{
		"data": fiber.Map{
			"track_save_count": trackSaveCount,
			"playlists":        playlists,
			"playlist_library": playlistLibrary,
			"user":             userWithoutLibrary,
		},
	})
}
