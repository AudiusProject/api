package api

import (
	"net/http/httptest"
	"testing"

	"bridgerton.audius.co/trashid"
	"github.com/stretchr/testify/assert"
)

func TestGetPlaylistStream(t *testing.T) {
	app := testAppWithFixtures(t)
	req := httptest.NewRequest("GET", "/v1/playlists/"+trashid.MustEncodeHashID(1)+"/stream", nil)
	res, err := app.Test(req, -1)
	assert.NoError(t, err)
	assert.Equal(t, 200, res.StatusCode)
	assert.Equal(t, "application/vnd.apple.mpegurl", res.Header.Get("Content-Type"))

	// Read response body
	body := make([]byte, res.ContentLength)
	res.Body.Read(body)
	bodyStr := string(body)

	// Check M3U8 format
	assert.Contains(t, bodyStr, "#EXTM3U")
	assert.Contains(t, bodyStr, "#EXT-X-VERSION:3")
	assert.Contains(t, bodyStr, "#EXT-X-TARGETDURATION:")
	assert.Contains(t, bodyStr, "#EXT-X-ENDLIST")
	assert.Contains(t, bodyStr, "#EXTINF:")
}
