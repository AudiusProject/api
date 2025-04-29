package api

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestResolveTrackURL(t *testing.T) {
	// Test successful track resolution
	status, _ := testGet(t, "/v1/resolve?url=https://audius.co/TracksByPermalink/track-by-permalink")
	assert.Equal(t, 302, status)

	// Test failed track resolution
	status, _ = testGet(t, "/v1/resolve?url=https://audius.co/nonexistent/track")
	assert.Equal(t, 404, status)
	status, _ = testGet(t, "/v1/resolve?url=invalid-url")
	assert.Equal(t, 404, status)
	status, _ = testGet(t, "/v1/resolve")
	assert.Equal(t, 400, status)
}

func TestResolvePlaylistURL(t *testing.T) {
	// Test successful playlist resolution
	status, _ := testGet(t, "/v1/resolve?url=https://audius.co/PlaylistsByPermalink/playlist/playlist-by-permalink")
	assert.Equal(t, 302, status)

	// Test successful album resolution
	status, _ = testGet(t, "/v1/resolve?url=https://audius.co/AlbumsByPermalink/album/album-by-permalink")
	assert.Equal(t, 302, status)

	// Test failed playlist resolution
	status, _ = testGet(t, "/v1/resolve?url=https://audius.co/nonexistent/playlist/playlist")
	assert.Equal(t, 404, status)
}

func TestResolveUserURL(t *testing.T) {
	// Test successful user resolution
	status, _ := testGet(t, "/v1/resolve?url=https://audius.co/rayjacobson")
	assert.Equal(t, 302, status)

	// Test failed user resolution
	status, _ = testGet(t, "/v1/resolve?url=https://audius.co/nonexistentuser")
	assert.Equal(t, 404, status)
}
