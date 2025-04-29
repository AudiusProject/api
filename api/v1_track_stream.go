package api

import (
	"net/http"
	"net/url"
	"time"

	"bridgerton.audius.co/api/dbv1"
	"github.com/gofiber/fiber/v2"
)

// tryFindWorkingUrl attempts to validate a media link by checking if it can serve content.
// It tries the primary URL first, then falls back to mirrors if needed.
// Returns the first valid URL found or the main URL if nothing works.
func tryFindWorkingUrl(mediaLink *dbv1.MediaLink) *url.URL {
	mainURL, err := url.Parse(mediaLink.Url)
	if err != nil {
		return nil
	}

	// Construct all URLs to try
	urls := make([]*url.URL, 0, len(mediaLink.Mirrors)+1)
	urls = append(urls, mainURL)
	for _, mirror := range mediaLink.Mirrors {
		mirrorURL := *mainURL
		mirrorURL.Host = mirror
		urls = append(urls, &mirrorURL)
	}

	client := &http.Client{
		Timeout: 5 * time.Second,
	}
	for _, u := range urls {
		q := u.Query()
		q.Set("skip_play_count", "true")
		u.RawQuery = q.Encode()

		req, err := http.NewRequest("GET", u.String(), nil)
		if err != nil {
			continue
		}
		req.Header.Set("Range", "bytes=0-1")

		resp, err := client.Do(req)
		if err != nil {
			continue
		}
		resp.Body.Close()

		if resp.StatusCode == http.StatusPartialContent ||
			resp.StatusCode == http.StatusOK ||
			resp.StatusCode == http.StatusNoContent {
			return u
		}
	}

	return mainURL
}

func (app *ApiServer) v1TrackStream(c *fiber.Ctx) error {
	myId := app.getMyId(c)
	trackId := c.Locals("trackId").(int)

	tracks, err := app.queries.FullTracks(c.Context(), dbv1.GetTracksParams{
		MyID: myId,
		Ids:  []int32{int32(trackId)},
	})
	if err != nil {
		return err
	}

	if len(tracks) == 0 {
		return sendError(c, 404, "track not found")
	}

	track := tracks[0]

	streamURL := tryFindWorkingUrl(track.Stream)

	if skipPlayCount := c.Query("skip_play_count"); skipPlayCount != "" {
		q := streamURL.Query()
		q.Set("skip_play_count", skipPlayCount)
		streamURL.RawQuery = q.Encode()
	}

	return c.Redirect(streamURL.String(), fiber.StatusFound)
}
