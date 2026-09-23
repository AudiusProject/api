package api

import (
	"path"
	"strings"

	"api.audius.co/api/dbv1"
	"github.com/gofiber/fiber/v2"
)

func createFilename(track *dbv1.Track) string {
	// Downloads serve the original upload when the row has one, so use its name.
	if track.OrigFileCid.String != "" && track.OrigFilename.String != "" {
		return track.OrigFilename.String
	}

	// Otherwise the bytes are the mp3 transcode, so swap the recorded
	// filename's extension for .mp3. Titles are used as-is since they have no
	// real extension.
	if name := track.OrigFilename.String; name != "" {
		return strings.TrimSuffix(name, path.Ext(name)) + ".mp3"
	}
	return track.Title.String + ".mp3"
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

	// Same guard as the stream endpoint: a deleted track, or one whose owner is
	// no longer active, must not have its audio served here either.
	if !track.IsStreamable {
		return fiber.NewError(fiber.StatusNotFound, "track not found")
	}

	// track.Download is only set when the public may download. Fall back to an
	// owner-signed link so the artist or their manager can always fetch their
	// own file (used by the edit page's "Download File" and replace-file flows).
	downloadLink := track.Download
	if downloadLink == nil {
		downloadLink, err = app.ownerDownloadLink(c, &track)
		if err != nil {
			return err
		}
	}

	if downloadLink == nil {
		if !track.Access.Download {
			return fiber.NewError(fiber.StatusForbidden, "you are not allowed to download this track")
		}
		return fiber.NewError(fiber.StatusNotFound, "track is not downloadable")
	}

	downloadUrl := tryFindWorkingUrl(downloadLink)

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

// ownerDownloadLink signs a download link when the wallet recovered from the
// request signature owns the track or holds an approved grant from the owner.
// It checks the signature rather than the user_id query param so it does not
// depend on authMiddleware's user_id handling for this route. Returns nil for
// anyone else.
func (app *ApiServer) ownerDownloadLink(c *fiber.Ctx, track *dbv1.Track) (*dbv1.MediaLink, error) {
	wallet := app.tryGetAuthedWallet(c)
	if wallet == "" {
		return nil, nil
	}

	ownerId := track.GetTracksRow.UserID
	if !app.isAuthorizedRequest(c.Context(), ownerId, wallet) {
		return nil, nil
	}

	return track.SignDownloadLink(ownerId)
}
