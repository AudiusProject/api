package api

import (
	"context"
	"testing"

	"api.audius.co/api/dbv1"
	"api.audius.co/trashid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPlaylistsEndpoint(t *testing.T) {
	app := testAppWithFixtures(t)
	status, body := testGet(t, app, "/v1/full/playlists?id=7eP5n")
	assert.Equal(t, 200, status)

	jsonAssert(t, body, map[string]any{
		"data.0.id":            "7eP5n",
		"data.0.playlist_name": "First",
		"data.0.tracks.#":      0,
	})
}

func TestPlaylistsEndpointWithTracks(t *testing.T) {
	app := testAppWithFixtures(t)
	status, body := testGet(t, app, "/v1/full/playlists?id=7eP5n&with_tracks=true")
	assert.Equal(t, 200, status)

	jsonAssert(t, body, map[string]any{
		"data.0.id":            "7eP5n",
		"data.0.playlist_name": "First",
		"data.0.tracks.#":      2,
	})
}

func TestPlaylistsEndpointWithPlaylistPermalink(t *testing.T) {
	app := testAppWithFixtures(t)
	var resp struct {
		Data []dbv1.Playlist
	}

	status, body := testGet(t, app, "/v1/full/playlists?permalink=/PlaylistsByPermalink/playlist/playlist-by-permalink", &resp)
	assert.Equal(t, 200, status)

	jsonAssert(t, body, map[string]any{
		"data.0.id":            "eYake",
		"data.0.playlist_name": "playlist by permalink",
	})
}

func TestPlaylistsEndpointWithAlbumPermalink(t *testing.T) {
	app := testAppWithFixtures(t)
	var resp struct {
		Data []dbv1.Playlist
	}

	status, body := testGet(t, app, "/v1/full/playlists?permalink=/AlbumsByPermalink/album/album-by-permalink", &resp)
	assert.Equal(t, 200, status)

	jsonAssert(t, body, map[string]any{
		"data.0.id":            "ePVXL",
		"data.0.playlist_name": "album by permalink",
	})
}

// A permalink-based lookup of a private playlist works for anonymous callers.
func TestPlaylistsEndpointPrivatePermalinkAnonymous(t *testing.T) {
	app := testAppWithFixtures(t)
	ctx := context.Background()
	require.NotNil(t, app.writePool, "test requires write pool")

	_, err := app.writePool.Exec(ctx, `UPDATE playlists SET is_private = true WHERE playlist_id = 500 AND is_current = true`)
	require.NoError(t, err)

	var resp struct {
		Data []dbv1.Playlist
	}
	status, body := testGet(t, app, "/v1/full/playlists?permalink=/PlaylistsByPermalink/playlist/playlist-by-permalink", &resp)
	assert.Equal(t, 200, status)
	assert.Len(t, resp.Data, 1, "permalink lookup must return private playlist even without auth")

	jsonAssert(t, body, map[string]any{
		"data.0.id":            "eYake",
		"data.0.playlist_name": "playlist by permalink",
		"data.0.is_private":    true,
	})
}

// An ID-based lookup must NOT return private playlists to anonymous callers.
func TestPlaylistsEndpointPrivateByIdHiddenFromAnonymous(t *testing.T) {
	app := testAppWithFixtures(t)
	ctx := context.Background()
	require.NotNil(t, app.writePool, "test requires write pool")

	_, err := app.writePool.Exec(ctx, `UPDATE playlists SET is_private = true WHERE playlist_id = 500 AND is_current = true`)
	require.NoError(t, err)

	var resp struct {
		Data []dbv1.Playlist
	}
	status, _ := testGet(t, app, "/v1/full/playlists?id=eYake", &resp)
	assert.Equal(t, 200, status)
	assert.Len(t, resp.Data, 0, "private playlist must not be returned for ID-based anonymous lookup")
}

// The single playlist endpoint must also hide private playlists from anonymous callers.
func TestGetPlaylistPrivateAnonymous404(t *testing.T) {
	app := testAppWithFixtures(t)
	ctx := context.Background()
	require.NotNil(t, app.writePool, "test requires write pool")

	_, err := app.writePool.Exec(ctx, `UPDATE playlists SET is_private = true WHERE playlist_id = 500 AND is_current = true`)
	require.NoError(t, err)

	status, _ := testGet(t, app, "/v1/full/playlists/eYake")
	assert.Equal(t, 404, status, "private playlist must 404 for anonymous ID-based fetch")
}

// The single playlist endpoint must return private playlists to their owner.
func TestGetPlaylistPrivateOwnerAllowed(t *testing.T) {
	app := testAppWithFixtures(t)
	// user 7's fixture wallet has no test signature, so bypass the auth
	// middleware and let user_id alone identify the owner for this test.
	app.skipAuthCheck = true
	ctx := context.Background()
	require.NotNil(t, app.writePool, "test requires write pool")

	// playlist 500 is owned by user 7
	_, err := app.writePool.Exec(ctx, `UPDATE playlists SET is_private = true WHERE playlist_id = 500 AND is_current = true`)
	require.NoError(t, err)

	ownerId := trashid.MustEncodeHashID(7)
	status, body := testGet(t, app, "/v1/full/playlists/eYake?user_id="+ownerId)
	assert.Equal(t, 200, status, "owner must be able to view their own private playlist by ID")
	jsonAssert(t, body, map[string]any{
		"data.0.id":            "eYake",
		"data.0.playlist_name": "playlist by permalink",
		"data.0.is_private":    true,
	})
}
