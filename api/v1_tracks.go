package api

import (
	"bridgerton.audius.co/api/dbv1"
	"bridgerton.audius.co/trashid"
	"github.com/gofiber/fiber/v2"
)

func (app *ApiServer) v1Tracks(c *fiber.Ctx) error {
	myId, _ := trashid.DecodeHashId(c.Query("user_id"))
	ids := decodeIdList(c)
	authedUserId := app.getAuthedUserId(c)
	authedWallet := app.getAuthedWallet(c)

	// Add permalink ID mappings
	permalinks := queryMutli(c, "permalink")
	if len(permalinks) > 0 {
		handles := make([]string, len(permalinks))
		slugs := make([]string, len(permalinks))
		for i, permalink := range permalinks {
			if match := trackURLRegex.FindStringSubmatch(permalink); match != nil {
				handles[i] = match[1]
				slugs[i] = match[2]
				permalinks[i] = permalink
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

	tracks, err := app.queries.FullTracks(c.Context(), dbv1.FullTracksParams{
		GetTracksParams: dbv1.GetTracksParams{
			MyID: int32(myId),
			Ids:  ids,
		},
		AuthedUserId:        authedUserId,
		AuthedWallet:        authedWallet,
		IsAuthorizedRequest: app.isAuthorizedRequest,
	})
	if err != nil {
		return err
	}

	return v1TracksResponse(c, tracks)
}
