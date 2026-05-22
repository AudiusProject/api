package api

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestV1PlaylistByPermalink(t *testing.T) {
	app := testAppWithFixtures(t)
	status, body := testGet(t, app, "/v1/full/playlists/by_permalink/PlaylistsByPermalink/playlist-by-permalink")
	assert.Equal(t, 200, status)

	jsonAssert(t, body, map[string]any{
		"data.0.id":            "eYake",
		"data.0.playlist_name": "playlist by permalink",
	})
}

func TestV1AlbumByPermalink(t *testing.T) {
	app := testAppWithFixtures(t)
	status, body := testGet(t, app, "/v1/full/playlists/by_permalink/AlbumsByPermalink/album-by-permalink")
	assert.Equal(t, 200, status)

	jsonAssert(t, body, map[string]any{
		"data.0.id":            "ePVXL",
		"data.0.playlist_name": "album by permalink",
	})
}

// A private playlist should be returned to anonymous callers when fetched via
// permalink — "has the link" is sufficient permission.
func TestV1PrivatePlaylistByPermalinkAnonymous(t *testing.T) {
	app := testAppWithFixtures(t)
	ctx := context.Background()
	require.NotNil(t, app.writePool, "test requires write pool")

	_, err := app.writePool.Exec(ctx, `UPDATE playlists SET is_private = true WHERE playlist_id = 500 AND is_current = true`)
	require.NoError(t, err)

	status, body := testGet(t, app, "/v1/full/playlists/by_permalink/PlaylistsByPermalink/playlist-by-permalink")
	assert.Equal(t, 200, status)

	jsonAssert(t, body, map[string]any{
		"data.0.id":            "eYake",
		"data.0.playlist_name": "playlist by permalink",
		"data.0.is_private":    true,
	})
}

// Same for albums.
func TestV1PrivateAlbumByPermalinkAnonymous(t *testing.T) {
	app := testAppWithFixtures(t)
	ctx := context.Background()
	require.NotNil(t, app.writePool, "test requires write pool")

	_, err := app.writePool.Exec(ctx, `UPDATE playlists SET is_private = true WHERE playlist_id = 501 AND is_current = true`)
	require.NoError(t, err)

	status, body := testGet(t, app, "/v1/full/playlists/by_permalink/AlbumsByPermalink/album-by-permalink")
	assert.Equal(t, 200, status)

	jsonAssert(t, body, map[string]any{
		"data.0.id":            "ePVXL",
		"data.0.playlist_name": "album by permalink",
		"data.0.is_private":    true,
	})
}
