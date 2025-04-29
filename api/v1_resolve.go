package api

import (
	"net/url"
	"regexp"
	"strings"

	"bridgerton.audius.co/api/dbv1"
	"bridgerton.audius.co/trashid"
	"github.com/gofiber/fiber/v2"
)

var (
	trackURLRegex    = regexp.MustCompile(`^/(?P<handle>[^/]*)/(?P<slug>[^/]*)$`)
	playlistURLRegex = regexp.MustCompile(`/(?P<handle>[^/]*)/(?P<playlistType>playlist|album)/(?P<slug>[^/]*)$`)
	userURLRegex     = regexp.MustCompile(`^/(?P<handle>[^/]*)$`)
)

func (app *ApiServer) v1Resolve(c *fiber.Ctx) error {
	isFull := app.getIsFull(c)
	urlStr := c.Query("url")
	if urlStr == "" {
		return fiber.NewError(fiber.StatusBadRequest, "Missing url parameter")
	}

	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "Invalid URL")
	}

	// Strip out any preceding protocol & domain
	path := parsedURL.Path

	// Try to match track URL
	if match := trackURLRegex.FindStringSubmatch(path); match != nil {
		handle := strings.ToLower(match[1])
		slug := match[2]

		trackIds, err := app.queries.GetTrackIdsByPermalink(c.Context(), dbv1.GetTrackIdsByPermalinkParams{
			Handles:    []string{handle},
			Slugs:      []string{slug},
			Permalinks: []string{path},
		})
		if err != nil || len(trackIds) == 0 {
			return fiber.NewError(fiber.StatusNotFound, "Track not found")
		}

		trackId, err := trashid.EncodeHashId(int(trackIds[0]))
		if err != nil {
			return err
		}

		if isFull {
			return c.Redirect("/v1/full/tracks/"+trackId, fiber.StatusFound)
		}
		return c.Redirect("/v1/tracks/"+trackId, fiber.StatusFound)
	}

	// Try to match playlist URL
	if match := playlistURLRegex.FindStringSubmatch(path); match != nil {
		handle := strings.ToLower(match[1])
		slug := match[3]

		playlistIds, err := app.queries.GetPlaylistIdsByPermalink(c.Context(), dbv1.GetPlaylistIdsByPermalinkParams{
			Handles:    []string{handle},
			Slugs:      []string{slug},
			Permalinks: []string{path},
		})
		if err != nil || len(playlistIds) == 0 {
			return fiber.NewError(fiber.StatusNotFound, "Playlist not found")
		}

		playlistId, err := trashid.EncodeHashId(int(playlistIds[0]))
		if err != nil {
			return err
		}

		if isFull {
			return c.Redirect("/v1/full/playlists/"+playlistId, fiber.StatusFound)
		}
		return c.Redirect("/v1/playlists/"+playlistId, fiber.StatusFound)
	}

	// Try to match user URL
	if match := userURLRegex.FindStringSubmatch(path); match != nil {
		handle := strings.ToLower(match[1])

		rawUserId, err := app.queries.GetUserForHandle(c.Context(), handle)

		if err != nil {
			return fiber.NewError(fiber.StatusNotFound, "User not found")
		}

		userId, err := trashid.EncodeHashId(int(rawUserId))
		if err != nil {
			return err
		}

		if isFull {
			return c.Redirect("/v1/full/users/"+userId, fiber.StatusFound)
		}
		return c.Redirect("/v1/users/"+userId, fiber.StatusFound)
	}

	return fiber.NewError(fiber.StatusNotFound, "URL not found")
}
