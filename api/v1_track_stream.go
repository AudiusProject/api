package api

import (
	"api.audius.co/api/dbv1"
	"github.com/gofiber/fiber/v2"
)

func (app *ApiServer) v1TrackStream(c *fiber.Ctx) error {
	myId := app.getMyId(c)
	trackId := c.Locals("trackId").(int)

	params := dbv1.TracksParams{
		GetTracksParams: dbv1.GetTracksParams{
			MyID:            myId,
			Ids:             []int32{int32(trackId)},
			AuthedWallet:    app.tryGetAuthedWallet(c),
			IncludeUnlisted: true,
		},
	}

	// If a verified Solana wallet is present, pass it through so
	// GetBulkTrackAccess can check token gate balances for it.
	if solWallet := app.tryGetSolanaWallet(c); solWallet != "" {
		params.SolanaWallet = solWallet
	}

	tracks, err := app.queries.Tracks(c.Context(), params)
	if err != nil {
		return err
	}

	if len(tracks) == 0 {
		return fiber.NewError(fiber.StatusNotFound, "track not found")
	}

	track := tracks[0]

	if track.Access.Stream {
		return app.redirectToStream(c, track.Stream)
	}

	return fiber.NewError(fiber.StatusForbidden, "track not streamable")
}

func (app *ApiServer) redirectToStream(c *fiber.Ctx, stream *dbv1.MediaLink) error {
	streamURL := tryFindWorkingUrl(stream)

	if skipPlayCount := c.Query("skip_play_count"); skipPlayCount != "" {
		q := streamURL.Query()
		q.Set("skip_play_count", skipPlayCount)
		streamURL.RawQuery = q.Encode()
	}

	if c.QueryBool("no_redirect") {
		return c.JSON(fiber.Map{
			"data": streamURL.String(),
		})
	}

	return c.Redirect(streamURL.String(), fiber.StatusFound)
}
