package api

import (
	"bridgerton.audius.co/api/dbv1"
	"github.com/gofiber/fiber/v2"
)

func createFilename(track *dbv1.FullTrack) string {
	origFilename := track.OrigFilename
	if origFilename.String == "" {
		origFilename = track.Title
	}
	return origFilename.String
}

type trackDownloadParams struct {
	Filename string `query:"filename"`
}

func (app *ApiServer) v1TrackDownload(c *fiber.Ctx) error {
	myId := app.getMyId(c)
	trackId := c.Locals("trackId").(int)
	var params trackDownloadParams
	if err := c.QueryParser(&params); err != nil {
		return err
	}

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
	if !track.Access.Download {
		return fiber.NewError(fiber.StatusForbidden, "track not downloadable")
	}

	downloadUrl := tryFindWorkingUrl(track.Download)

	q := downloadUrl.Query()
	q.Set("skip_play_count", "true")
	if params.Filename != "" {
		q.Set("filename", params.Filename)
	} else {
		q.Set("filename", createFilename(&track))
	}
	downloadUrl.RawQuery = q.Encode()

	return c.Redirect(downloadUrl.String(), fiber.StatusFound)
}
