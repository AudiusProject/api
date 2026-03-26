package api

import (
	"api.audius.co/api/dbv1"
	"github.com/gofiber/fiber/v2"
)

func (app *ApiServer) v1TrackStream(c *fiber.Ctx) error {
	myId := app.getMyId(c)
	trackId := c.Locals("trackId").(int)

	tracks, err := app.queries.Tracks(c.Context(), dbv1.TracksParams{
		GetTracksParams: dbv1.GetTracksParams{
			MyID:            myId,
			Ids:             []int32{int32(trackId)},
			AuthedWallet:    app.tryGetAuthedWallet(c),
			IncludeUnlisted: true,
		},
	})
	if err != nil {
		return err
	}

	if len(tracks) == 0 {
		return fiber.NewError(fiber.StatusNotFound, "track not found")
	}

	track := tracks[0]

	// If standard access check passes, use the pre-built stream URL
	if track.Access.Stream {
		return app.redirectToStream(c, track.Stream)
	}

	// Fallback: check if a verified Solana wallet has sufficient token balance
	solWallet := app.tryGetSolanaWallet(c)
	if solWallet != "" && track.StreamConditions != nil && track.StreamConditions.TokenGate != nil {
		hasAccess, err := app.checkSolanaWalletTokenAccess(
			c.Context(),
			solWallet,
			track.StreamConditions.TokenGate.TokenMint,
			track.StreamConditions.TokenGate.TokenAmount,
		)
		if err == nil && hasAccess {
			// Build a stream URL on the fly since the track didn't include one
			// (stream URLs are only pre-built when standard access is granted)
			stream, err := dbv1.BuildMediaLink(
				track.TrackCid.String,
				track.TrackID,
				myId,
				&dbv1.Id3Tags{Title: track.Title.String, Artist: track.User.Name.String},
			)
			if err != nil {
				return err
			}
			return app.redirectToStream(c, stream)
		}
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
