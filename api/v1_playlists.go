package api

import (
	"strings"

	"bridgerton.audius.co/api/dbv1"
	"github.com/gofiber/fiber/v2"
)

func (app *ApiServer) v1playlists(c *fiber.Ctx) error {
	myId := app.getMyId(c)
	ids := decodeIdList(c)

	// Add permalink ID mappings
	permalinks := queryMutli(c, "permalink")
	if len(permalinks) > 0 {
		handles := make([]string, len(permalinks))
		slugs := make([]string, len(permalinks))
		for i, permalink := range permalinks {
			if match := playlistURLRegex.FindStringSubmatch(permalink); match != nil {
				handles[i] = strings.ToLower(match[1])
				slugs[i] = match[3]
				playlistType := match[2]
				permalinks[i] = handles[i] + "/" + playlistType + "/" + slugs[i]
			} else {
				return fiber.NewError(fiber.StatusBadRequest, "Invalid permalink: "+permalinks[i])
			}
		}
		newIds, err := app.queries.GetPlaylistIdsByPermalink(c.Context(), dbv1.GetPlaylistIdsByPermalinkParams{
			Handles:    handles,
			Slugs:      slugs,
			Permalinks: permalinks,
		})
		if err != nil {
			return err
		}
		ids = append(ids, newIds...)
	}

	playlists, err := app.queries.FullPlaylists(c.Context(), dbv1.GetPlaylistsParams{
		MyID: myId,
		Ids:  ids,
	})
	if err != nil {
		return err
	}

	return v1PlaylistsResponse(c, playlists)
}
