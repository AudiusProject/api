package api

import (
	"bridgerton.audius.co/api/dbv1"
	"github.com/gofiber/fiber/v2"
)

func (app *ApiServer) v1TrackStream(c *fiber.Ctx) error {
	myId := app.getMyId(c)
	trackId := c.Locals("trackId").(int)

	tracks, err := app.queries.FullTracks(c.Context(), dbv1.FullTracksParams{
		GetTracksParams: dbv1.GetTracksParams{
			MyID: myId,
			Ids:  []int32{int32(trackId)},
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

	q := streamURL.Query()
	if skipPlayCount := c.Query("skip_play_count"); skipPlayCount != "" {
		q.Set("skip_play_count", skipPlayCount)
	}

	// set id3 tags as query params in stream url
	// audiusd will set the id3 tags on the fly
	q.Set("id3", "true")
	q.Set("id3_artist", track.User.Handle.String)
	q.Set("id3_title", track.Title.String)

	streamURL.RawQuery = q.Encode()

	return c.Redirect(streamURL.String(), fiber.StatusFound)
}
