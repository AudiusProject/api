package api

import (
	"bridgerton.audius.co/api/dbv1"
	"github.com/gofiber/fiber/v2"
)

func (app *ApiServer) v1Track(c *fiber.Ctx) error {
	myId := app.getMyId(c)
	authedUserId := app.getAuthedUserId(c)
	authedWallet := app.getAuthedWallet(c)
	trackId := c.Locals("trackId").(int)

	tracks, err := app.queries.FullTracks(c.Context(), dbv1.FullTracksParams{
		GetTracksParams: dbv1.GetTracksParams{
			MyID: myId,
			Ids:  []int32{int32(trackId)},
		},
		AuthedUserId:        authedUserId,
		AuthedWallet:        authedWallet,
		IsAuthorizedRequest: app.isAuthorizedRequest,
	})
	if err != nil {
		return err
	}

	if len(tracks) == 0 {
		return sendError(c, 404, "track not found")
	}

	track := tracks[0]

	return v1TrackResponse(c, track)
}
