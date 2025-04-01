package api

import (
	"bridgerton.audius.co/queries"
	"bridgerton.audius.co/trashid"
	"github.com/gofiber/fiber/v2"
)

func (app *ApiServer) v1Tracks(c *fiber.Ctx) error {
	myId, _ := trashid.DecodeHashId(c.Query("user_id"))
	ids := decodeIdList(c)

	tracks, err := app.queries.FullTracks(c.Context(), queries.GetTracksParams{
		MyID: int32(myId),
		Ids:  ids,
	})
	if err != nil {
		return err
	}
	return c.JSON(fiber.Map{
		"data": tracks,
	})
}
