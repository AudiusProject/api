package api

import (
	"testing"

	"bridgerton.audius.co/api/dbv1"
	"github.com/stretchr/testify/assert"
)

func TestTracksEndpoint(t *testing.T) {
	app := testAppWithFixtures(t)
	var resp struct {
		Data []dbv1.FullTrack
	}

	status, body := testGet(t, app, "/v1/full/tracks?id=eYZmn", &resp)
	assert.Equal(t, 200, status)

	jsonAssert(t, body, map[string]any{
		"data.0.id":    "eYZmn",
		"data.0.title": "T1",
	})
}

func TestGetTracksByPermalink(t *testing.T) {
	app := testAppWithFixtures(t)

	status, body := testGet(t, app, "/v1/full/tracks?permalink=/TracksByPermalink/track-by-permalink")
	assert.Equal(t, 200, status)

	jsonAssert(t, body, map[string]any{
		"data.0.id":    "eYake",
		"data.0.title": "track by permalink",
	})
}
