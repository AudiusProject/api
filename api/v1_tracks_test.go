package api

import (
	"testing"

	"bridgerton.audius.co/api/dbv1"
	"github.com/stretchr/testify/assert"
)

func TestGetTracksByPermalink(t *testing.T) {
	var tracksResponse struct {
		Data []dbv1.FullTrack
	}

	status, body := testGet(t, "/v1/full/tracks?permalink=/tracksbypermalink/track-by-permalink", &tracksResponse)
	assert.Equal(t, 200, status)

	jsonAssert(t, body, map[string]string{
		"data.0.id":    "eYake",
		"data.0.title": "track by permalink",
	})
}
