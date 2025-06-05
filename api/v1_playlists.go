package api

import (
	"strconv"

	"bridgerton.audius.co/api/dbv1"
	"github.com/gofiber/fiber/v2"
)

func (app *ApiServer) v1Playlists(c *fiber.Ctx) error {
	myId := app.getMyId(c)
	ids := decodeIdList(c)

	// by default the bulk playlist endpoint will omit tracks
	// unless client explicitly does ?with_tracks=true
	withTracks, _ := strconv.ParseBool(c.Query("with_tracks", "false"))

	// Add permalink ID mappings
	permalinks := queryMutli(c, "permalink")
	if len(permalinks) > 0 {
		handles := make([]string, len(permalinks))
		slugs := make([]string, len(permalinks))
		for i, permalink := range permalinks {
			if match := playlistURLRegex.FindStringSubmatch(permalink); match != nil {
				handles[i] = match[1]
				slugs[i] = match[3]
				permalinks[i] = permalink
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

	playlists, err := app.queries.FullPlaylists(c.Context(), dbv1.FullPlaylistsParams{
		GetPlaylistsParams: dbv1.GetPlaylistsParams{
			MyID: myId,
			Ids:  ids,
		},
		OmitTracks: !withTracks,
	})
	if err != nil {
		return err
	}

	return v1PlaylistsResponse(c, playlists)
}
