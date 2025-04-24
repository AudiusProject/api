package api

import (
	"bridgerton.audius.co/api/dbv1"
	"github.com/gofiber/fiber/v2"
)

func v1UserResponse(c *fiber.Ctx, user dbv1.FullUser) error {
	if c.Locals("isFull").(bool) {
		return c.JSON(fiber.Map{
			"data": user,
		})
	}
	return c.JSON(fiber.Map{
		"data": dbv1.ToMinUser(user),
	})
}

func v1UsersResponse(c *fiber.Ctx, users []dbv1.FullUser) error {
	if c.Locals("isFull").(bool) {
		return c.JSON(fiber.Map{
			"data": users,
		})
	}
	return c.JSON(fiber.Map{
		"data": dbv1.ToMinUsers(users),
	})
}

// Note: playlist response returned an array even though it's a single playlist
// Done for backwards compatibility. Would be nice to get rid of this.
func v1PlaylistResponse(c *fiber.Ctx, playlist dbv1.FullPlaylist) error {
	if c.Locals("isFull").(bool) {
		return c.JSON(fiber.Map{
			"data": []dbv1.FullPlaylist{playlist},
		})
	}
	return c.JSON(fiber.Map{
		"data": []dbv1.MinPlaylist{dbv1.ToMinPlaylist(playlist)},
	})
}

func v1PlaylistsResponse(c *fiber.Ctx, playlists []dbv1.FullPlaylist) error {
	if c.Locals("isFull").(bool) {
		return c.JSON(fiber.Map{
			"data": playlists,
		})
	}
	return c.JSON(fiber.Map{
		"data": dbv1.ToMinPlaylists(playlists),
	})
}

func v1TrackResponse(c *fiber.Ctx, track dbv1.FullTrack) error {
	if c.Locals("isFull").(bool) {
		return c.JSON(fiber.Map{
			"data": track,
		})
	}
	return c.JSON(fiber.Map{
		"data": dbv1.ToMinTrack(track),
	})
}

func v1TracksResponse(c *fiber.Ctx, tracks []dbv1.FullTrack) error {
	if c.Locals("isFull").(bool) {
		return c.JSON(fiber.Map{
			"data": tracks,
		})
	}
	return c.JSON(fiber.Map{
		"data": dbv1.ToMinTracks(tracks),
	})
}
