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

	tracks, err := app.queries.Tracks(c.Context(), params)
	if err != nil {
		return err
	}

	if len(tracks) == 0 {
		return fiber.NewError(fiber.StatusNotFound, "track not found")
	}

	track := tracks[0]

	// `is_streamable` is false when the track is deleted or its owner is no
	// longer active - either the artist deactivated their own account or the
	// account was delisted by the trusted notifier. The track response has
	// always reported this, but nothing enforced it, so the audio stayed
	// reachable to anyone holding the URL. Treat it as not found rather than
	// forbidden so we don't distinguish these from a missing track.
	if !track.IsStreamable {
		return fiber.NewError(fiber.StatusNotFound, "track not found")
	}

	if track.Access.Stream {
		// Stream is nil when the track row has no cid to sign (e.g. an
		// upload-v2 row that never got its track_cid backfilled).
		if track.Stream == nil {
			return fiber.NewError(fiber.StatusNotFound, "track audio is unavailable")
		}
		return app.redirectToStream(c, track.Stream)
	}

	return fiber.NewError(fiber.StatusForbidden, "track not streamable")
}

func (app *ApiServer) redirectToStream(c *fiber.Ctx, stream *dbv1.MediaLink) error {
	// Temporary, for the genesis migration: prefer hosts that stay on the old
	// chain so plays are not split across two chains while the fleet migrates.
	// No-op when unconfigured. See withPlayRoutingHosts.
	stream = withPlayRoutingHosts(stream, app.config.PlayRoutingHosts)

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
