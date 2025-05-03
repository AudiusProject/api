package api

import (
	"bridgerton.audius.co/api/dbv1"
	"github.com/gofiber/fiber/v2"
)

func (app *ApiServer) v1TrackDownload(c *fiber.Ctx) error {
	myId := app.getMyId(c)
	trackId := c.Locals("trackId").(int)
	filename := c.Query("filename")

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
		return sendError(c, 404, "track not found")
	}

	track := tracks[0]
	if !track.Access.Download {
		return sendError(c, 403, "track not downloadable")
	}

	downloadUrl := tryFindWorkingUrl(track.Download)

	q := downloadUrl.Query()
	q.Set("skip_play_count", "true")
	if filename != "" {
		q.Set("filename", filename)
	}
	downloadUrl.RawQuery = q.Encode()

	return c.Redirect(downloadUrl.String(), fiber.StatusFound)
}
