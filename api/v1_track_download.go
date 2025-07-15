package api

import (
	"path/filepath"
	"strings"

	"bridgerton.audius.co/api/dbv1"
	"github.com/gofiber/fiber/v2"
)

func createFilename(track *dbv1.FullTrack, isOriginal bool) string {
	origFilename := track.OrigFilename
	if origFilename.String == "" {
		origFilename = track.Title
	}

	if isOriginal {
		return origFilename.String
	}

	// Remove extension and add .mp3
	ext := filepath.Ext(origFilename.String)
	nameWithoutExt := strings.TrimSuffix(origFilename.String, ext)
	return nameWithoutExt + ".mp3"
}

func (app *ApiServer) v1TrackDownload(c *fiber.Ctx) error {
	myId := app.getMyId(c)
	trackId := c.Locals("trackId").(int)
	filename := c.Query("filename")
	isOriginal := c.Query("is_original") == "true"

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
	if filename != "" {
		q.Set("filename", filename)
	} else {
		q.Set("filename", createFilename(&track, isOriginal))
	}
	downloadUrl.RawQuery = q.Encode()

	return c.Redirect(downloadUrl.String(), fiber.StatusFound)
}
