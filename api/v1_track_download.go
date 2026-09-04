package api

import (
	"path"
	"strings"

	"api.audius.co/api/dbv1"
	"github.com/gofiber/fiber/v2"
)

func createFilename(track *dbv1.Track) string {
	// The original upload is what a download serves whenever the row kept one,
	// so its own name is the right one.
	if track.OrigFileCid.String != "" && track.OrigFilename.String != "" {
		return track.OrigFilename.String
	}

	// Otherwise the bytes are the mp3 transcode, and the name must not promise
	// the format the artist uploaded: a .wav name on mp3 bytes is a file most
	// editors refuse to open. Only the recorded filename is stripped of its
	// extension - a title is free text and "Vol. 2" has no extension to trim.
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

	// track.Download is only populated for tracks the public may download, so
	// an artist who left downloads off - about four in five tracks - could not
	// get their own file back. The edit page's "Download File" button and the
	// replace-file flow both come through here, and both were 404ing for the
	// owner of the track, surfacing as a generic "something went wrong".
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

// ownerDownloadLink signs a download link for a requester who has proven they
// own the track, or manage the account that does. Ownership comes from the
// wallet recovered from the request signature, not from the user_id query
// param behind myId: user_id is the caller's own claim, and it is only
// trustworthy here because this route sits off authMiddleware's advisory-
// user_id allowlist. Handing out an artist's original master should not rest
// on that list continuing to exclude this route. Returns nil - not an error -
// for everyone else, which leaves the caller's 404 in place.
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
