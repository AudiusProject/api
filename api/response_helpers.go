package api

import (
	"api.audius.co/api/dbv1"
	"github.com/gofiber/fiber/v2"
)

func v1UserResponse(c *fiber.Ctx, user dbv1.User) error {
	if c.Locals("isFull").(bool) {
		return c.JSON(fiber.Map{
			"data": user,
		})
	}
	return c.JSON(fiber.Map{
		"data": user,
	})
}

func v1UsersResponse(c *fiber.Ctx, users []dbv1.User) error {
	if c.Locals("isFull").(bool) {
		return c.JSON(fiber.Map{
			"data": users,
		})
	}
	return c.JSON(fiber.Map{
		"data": users,
	})
}

// Note: playlist response returned an array even though it's a single playlist
// Done for backwards compatibility. Would be nice to get rid of this.
// Default and full both return full playlist shape (min-collection parity with full).
func v1PlaylistResponse(c *fiber.Ctx, playlist dbv1.Playlist) error {
	return c.JSON(fiber.Map{
		"data": []dbv1.Playlist{playlist},
	})
}

func v1PlaylistsResponse(c *fiber.Ctx, playlists []dbv1.Playlist) error {
	return c.JSON(fiber.Map{
		"data": playlists,
	})
}

func v1TrackResponse(c *fiber.Ctx, track dbv1.Track) error {
	if c.Locals("isFull").(bool) {
		return c.JSON(fiber.Map{
			"data": track,
		})
	}
	return c.JSON(fiber.Map{
		"data": track,
	})
}

func v1TracksResponse(c *fiber.Ctx, tracks []dbv1.Track) error {
	if c.Locals("isFull").(bool) {
		return c.JSON(fiber.Map{
			"data": tracks,
		})
	}
	return c.JSON(fiber.Map{
		"data": tracks,
	})
}

func v1TipsResponse(c *fiber.Ctx, tips []dbv1.FullTip) error {
	if c.Locals("isFull").(bool) {
		return c.JSON(fiber.Map{
			"data": tips,
		})
	}
	return c.JSON(fiber.Map{
		"data": dbv1.ToMinTips(tips),
	})
}
