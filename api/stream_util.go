package api

import (
	"net/http"
	"net/url"
	"time"

	"api.audius.co/api/dbv1"
)

// tryFindWorkingUrl attempts to validate a media link by checking if it can serve content.
// It tries the primary URL first, then falls back to mirrors if needed.
// Returns the first valid URL found or the main URL if nothing works.
//
// The returned URL carries no probe artifacts: callers hand it straight to a
// client, and a stray skip_play_count would stop the serving node from
// recording the listen.
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
		mirrorHostURL, err := url.Parse(mirror)
		if err != nil {
			continue
		}
		mirrorURL.Host = mirrorHostURL.Host
		urls = append(urls, &mirrorURL)
	}

	client := &http.Client{
		Timeout: 5 * time.Second,
	}
	for _, u := range urls {
		// Probe on a COPY. skip_play_count exists so this two-byte probe is not
		// counted as a listen, but it belongs to the probe alone -- mutating u
		// would carry the flag into the URL we hand the client, and the node
		// serving /tracks/cidstream/:cid returns early from logTrackListen when
		// it sees it. That silently suppressed the play for every caller of
		// /v1/tracks/:id/stream.
		probe := *u
		q := probe.Query()
		q.Set("skip_play_count", "true")
		probe.RawQuery = q.Encode()

		req, err := http.NewRequest("GET", probe.String(), nil)
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
