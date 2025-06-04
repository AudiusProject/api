package api

import (
	"testing"

	"bridgerton.audius.co/api/dbv1"
	"bridgerton.audius.co/trashid"
	"github.com/stretchr/testify/assert"
)

func TestGetTrendingPlaylists(t *testing.T) {
	app := testAppWithFixtures(t)
	var resp struct {
		Data []dbv1.FullPlaylist
	}
	status, body := testGet(t, app, "/v1/playlists/trending", &resp)

	assert.Equal(t, 200, status)

	jsonAssert(t, body, map[string]any{
		"data.0.id": trashid.MustEncodeHashID(1),
		"data.1.id": trashid.MustEncodeHashID(500),
	})

	// These playlists fall outside of params
	for _, playlist := range resp.Data {
		assert.NotEqual(t, trashid.MustEncodeHashID(2), playlist.PlaylistID, "Playlist 2 should not be in result set")
	}
}
