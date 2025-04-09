package api

import (
	"bridgerton.audius.co/api/dbv1"
	"github.com/gofiber/fiber/v2"
)

func v1UserResponse(c *fiber.Ctx, users []dbv1.FullUser) error {
	if c.Locals("isFull").(bool) {
		return c.JSON(fiber.Map{
			"data": users,
		})
	}
	return c.JSON(fiber.Map{
		"data": dbv1.ToMinUsers(users),
	})
}

func v1PlaylistResponse(c *fiber.Ctx, playlists []dbv1.FullPlaylist) error {
	if c.Locals("isFull").(bool) {
		return c.JSON(fiber.Map{
			"data": playlists,
		})
	}
	return c.JSON(fiber.Map{
		"data": dbv1.ToMinPlaylists(playlists),
	})
}

func v1TrackResponse(c *fiber.Ctx, tracks []dbv1.FullTrack) error {
	if c.Locals("isFull").(bool) {
		return c.JSON(fiber.Map{
			"data": tracks,
		})
	}
	return c.JSON(fiber.Map{
		"data": dbv1.ToMinTracks(tracks),
	})
}
