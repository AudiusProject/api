package api

import (
	"strings"

	"bridgerton.audius.co/api/dbv1"
	"bridgerton.audius.co/trashid"
	"github.com/gofiber/fiber/v2"
)

func (app *ApiServer) v1Tracks(c *fiber.Ctx) error {
	myId, _ := trashid.DecodeHashId(c.Query("user_id"))
	ids := decodeIdList(c)

	// Add permalink ID mappings
	permalinks := queryMutli(c, "permalink")
	if len(permalinks) > 0 {
		handles := make([]string, len(permalinks))
		slugs := make([]string, len(permalinks))
		for i, permalink := range permalinks {
			permalinks[i] = strings.TrimPrefix(permalink, "/")
			splits := strings.Split(permalinks[i], "/")
			handles[i] = splits[0]
			slugs[i] = splits[1]
		}
		newIds, err := app.queries.GetTrackIdsByPermalink(c.Context(), dbv1.GetTrackIdsByPermalinkParams{
			Handles:    handles,
			Slugs:      slugs,
			Permalinks: permalinks,
		})
		if err != nil {
			return err
		}
		ids = append(ids, newIds...)
	}

	tracks, err := app.queries.FullTracks(c.Context(), dbv1.GetTracksParams{
		MyID: int32(myId),
		Ids:  ids,
	})
	if err != nil {
		return err
	}

	return v1TracksResponse(c, tracks)
}
