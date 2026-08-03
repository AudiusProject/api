package api

import (
	"strconv"

	"api.audius.co/api/dbv1"
	"github.com/gofiber/fiber/v2"
)

func (app *ApiServer) v1Playlists(c *fiber.Ctx) error {
	myId := app.getMyId(c)
	ids := decodeIdList(c)

	// by default the bulk playlist endpoint will omit tracks
	// unless client explicitly does ?with_tracks=true
	withTracks, _ := strconv.ParseBool(c.Query("with_tracks", "false"))

	authedWallet := app.tryGetAuthedWallet(c)

	// Permalink-matched IDs are kept separate from caller-supplied IDs. Possession
	// of a valid permalink is treated as proof of access, so private playlists
	// matched by permalink are returned without auth. We must not extend that
	// trust to user-supplied IDs in the same request.
	var permalinkIds []int32
	permalinks := queryMulti(c, "permalink")
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
		var err error
		permalinkIds, err = app.queries.GetPlaylistIdsByPermalink(c.Context(), dbv1.GetPlaylistIdsByPermalinkParams{
			Handles:    handles,
			Slugs:      slugs,
			Permalinks: permalinks,
		})
		if err != nil {
			return err
		}
	}

	upcs := queryMulti(c, "upc")
	if len(upcs) > 0 {
		newIds, err := app.queries.GetPlaylistIdsByUPC(c.Context(), upcs)
		if err != nil {
			return err
		}
		ids = append(ids, newIds...)
	}

	out := make([]dbv1.Playlist, 0, len(ids)+len(permalinkIds))

	if len(ids) > 0 {
		playlists, err := app.queries.Playlists(c.Context(), dbv1.PlaylistsParams{
			GetPlaylistsParams: dbv1.GetPlaylistsParams{
				MyID: myId,
				Ids:  ids,
			},
			OmitTracks:   !withTracks,
			AuthedWallet: authedWallet,
		})
		if err != nil {
			return err
		}
		out = append(out, playlists...)
	}

	if len(permalinkIds) > 0 {
		playlists, err := app.queries.Playlists(c.Context(), dbv1.PlaylistsParams{
			GetPlaylistsParams: dbv1.GetPlaylistsParams{
				MyID:           myId,
				Ids:            permalinkIds,
				IncludePrivate: true,
			},
			OmitTracks:   !withTracks,
			AuthedWallet: authedWallet,
		})
		if err != nil {
			return err
		}
		out = append(out, playlists...)
	}

	return v1PlaylistsResponse(c, out)
}
