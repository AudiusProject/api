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
	if !track.Access.Stream {
		return fiber.NewError(fiber.StatusForbidden, "track not streamable")
	}

	streamURL := tryFindWorkingUrl(track.Stream)

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
