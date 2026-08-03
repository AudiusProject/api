package api

import (
	"net/url"
	"regexp"
	"strings"

	"api.audius.co/api/dbv1"
	"api.audius.co/trashid"
	"github.com/gofiber/fiber/v2"
)

// Params to preserve when redirecting (e.g. app_name, api_key for rate limiting/attribution)
var resolvePreservedParams = map[string]bool{
	"app_name": true,
	"api_key":  true,
}

// redirectWithPreservedParams redirects to path with app_name, api_key, etc. preserved from the request.
func (app *ApiServer) redirectWithPreservedParams(c *fiber.Ctx, path string, status int) error {
	q := make(url.Values)
	for k := range resolvePreservedParams {
		if v := c.Query(k); v != "" {
			q.Set(k, v)
		}
	}
	if len(q) > 0 {
		path = path + "?" + q.Encode()
	}
	return c.Redirect(path, status)
}

var (
	trackURLRegex    = regexp.MustCompile(`^/?(?P<handle>[^/]+)/(?P<slug>[^/]+)$`)
	playlistURLRegex = regexp.MustCompile(`/?(?P<handle>[^/]+)/(?P<playlistType>playlist|album)/(?P<slug>[^/]+)$`)
	userURLRegex     = regexp.MustCompile(`^/?(?P<handle>[^/]+)$`)
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
			return app.redirectWithPreservedParams(c, "/v1/full/tracks/"+trackId, fiber.StatusFound)
		}
		return app.redirectWithPreservedParams(c, "/v1/tracks/"+trackId, fiber.StatusFound)
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

		// Redirect to the by_permalink route so the destination handler can
		// honor "has the link" as access to private playlists/albums.
		if isFull {
			return app.redirectWithPreservedParams(c, "/v1/full/playlists/by_permalink/"+handle+"/"+slug, fiber.StatusFound)
		}
		return app.redirectWithPreservedParams(c, "/v1/playlists/by_permalink/"+handle+"/"+slug, fiber.StatusFound)
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
			return app.redirectWithPreservedParams(c, "/v1/full/users/"+userId, fiber.StatusFound)
		}
		return app.redirectWithPreservedParams(c, "/v1/users/"+userId, fiber.StatusFound)
	}

	return fiber.NewError(fiber.StatusNotFound, "URL not found")
}
