package api

import (
	"strings"

	"api.audius.co/api/dbv1"
	"api.audius.co/trashid"
	"github.com/gofiber/fiber/v2"
)

func (app *ApiServer) v1Tracks(c *fiber.Ctx) error {
	myId, _ := trashid.DecodeHashId(c.Query("user_id"))
	ids := decodeIdList(c)

	// Add permalink ID mappings
	permalinks := queryMulti(c, "permalink")
	if len(permalinks) > 0 {
		handles := make([]string, len(permalinks))
		slugs := make([]string, len(permalinks))
		for i, permalink := range permalinks {
			if match := trackURLRegex.FindStringSubmatch(permalink); match != nil {
				handles[i] = match[1]
				slugs[i] = match[2]
				// Add leading slash to permalink
				permalinks[i] = "/" + strings.TrimPrefix(permalink, "/")
			} else {
				return fiber.NewError(fiber.StatusBadRequest, "Invalid permalink: "+permalink)
			}
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

	// Add ISRC ID mappings
	isrcs := queryMulti(c, "isrc")
	if len(isrcs) > 0 {
		newIds, err := app.queries.GetTrackIdsByISRC(c.Context(), isrcs)
		if err != nil {
			return err
		}
		ids = append(ids, newIds...)
	}

	tracks, err := app.queries.Tracks(c.Context(), dbv1.TracksParams{
		GetTracksParams: dbv1.GetTracksParams{
			MyID:          int32(myId),
			Ids:           ids,
			IncludeUnlisted: true,
			AuthedWallet:  app.getAuthedWalletOptional(c),
		},
	})
	if err != nil {
		return err
	}

	return v1TracksResponse(c, tracks)
}
