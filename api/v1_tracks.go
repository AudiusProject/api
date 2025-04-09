package api

import (
	"bridgerton.audius.co/api/dbv1"
	"bridgerton.audius.co/trashid"
	"github.com/gofiber/fiber/v2"
)

func (app *ApiServer) v1Tracks(c *fiber.Ctx) error {
	myId, _ := trashid.DecodeHashId(c.Query("user_id"))
	ids := decodeIdList(c)

	tracks, err := app.queries.FullTracks(c.Context(), dbv1.GetTracksParams{
		MyID: int32(myId),
		Ids:  ids,
	})
	if err != nil {
		return err
	}

	return v1TrackResponse(c, tracks)
}
