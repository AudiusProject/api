package api

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolveTrackURL(t *testing.T) {
	app := testAppWithFixtures(t)
	// Test successful track resolution
	status, _ := testGet(t, app, "/v1/resolve?url=https://audius.co/TracksByPermalink/track-by-permalink")
	assert.Equal(t, 302, status)

	// Resolve does not filter by access_authorities (no tracks join); filter happens in GetTracks.
	// So a gated track still resolves (302), but fetching the track by ID returns empty.
	require.NotNil(t, app.writePool, "test requires write pool")
	_, err := app.writePool.Exec(context.Background(), `UPDATE tracks SET access_authorities = ARRAY['0xabc']::text[] WHERE track_id = 500 AND is_current = true`)
	require.NoError(t, err)
	status, _ = testGet(t, app, "/v1/resolve?url=https://audius.co/TracksByPermalink/track-by-permalink")
	assert.Equal(t, 302, status, "resolve still redirects; GetTracks filters gated tracks when fetching by ID")

	// Test failed track resolution
	status, _ = testGet(t, app, "/v1/resolve?url=https://audius.co/nonexistent/track")
	assert.Equal(t, 404, status)
	status, _ = testGet(t, app, "/v1/resolve?url=invalid-url")
	assert.Equal(t, 404, status)
	status, _ = testGet(t, app, "/v1/resolve")
	assert.Equal(t, 400, status)
}

func TestResolvePlaylistURL(t *testing.T) {
	app := testAppWithFixtures(t)
	// Test successful playlist resolution
	status, _ := testGet(t, app, "/v1/resolve?url=https://audius.co/PlaylistsByPermalink/playlist/playlist-by-permalink")
	assert.Equal(t, 302, status)

	// Test successful album resolution
	status, _ = testGet(t, app, "/v1/resolve?url=https://audius.co/AlbumsByPermalink/album/album-by-permalink")
	assert.Equal(t, 302, status)

	// Test failed playlist resolution
	status, _ = testGet(t, app, "/v1/resolve?url=https://audius.co/nonexistent/playlist/playlist")
	assert.Equal(t, 404, status)
}

func TestResolveUserURL(t *testing.T) {
	app := testAppWithFixtures(t)
	// Test successful user resolution
	status, _ := testGet(t, app, "/v1/resolve?url=https://audius.co/rayjacobson")
	assert.Equal(t, 302, status)

	// Test failed user resolution
	status, _ = testGet(t, app, "/v1/resolve?url=https://audius.co/nonexistentuser")
	assert.Equal(t, 404, status)
}
