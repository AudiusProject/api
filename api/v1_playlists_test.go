package api

import (
	"testing"

	"bridgerton.audius.co/api/dbv1"
	"github.com/stretchr/testify/assert"
)

func TestPlaylistsEndpoint(t *testing.T) {
	status, body := testGet(t, "/v1/full/playlists?id=7eP5n")
	assert.Equal(t, 200, status)

	jsonAssert(t, body, map[string]any{
		"data.0.id":            "7eP5n",
		"data.0.playlist_name": "First",
		"data.0.tracks.#":      0,
	})
}

func TestPlaylistsEndpointWithTracks(t *testing.T) {
	status, body := testGet(t, "/v1/full/playlists?id=7eP5n&with_tracks=true")
	assert.Equal(t, 200, status)

	jsonAssert(t, body, map[string]any{
		"data.0.id":            "7eP5n",
		"data.0.playlist_name": "First",
		"data.0.tracks.#":      2,
	})
}

func TestPlaylistsEndpointWithPlaylistPermalink(t *testing.T) {
	var resp struct {
		Data []dbv1.FullPlaylist
	}

	status, body := testGet(t, "/v1/full/playlists?permalink=/PlaylistsByPermalink/playlist/playlist-by-permalink", &resp)
	assert.Equal(t, 200, status)

	jsonAssert(t, body, map[string]any{
		"data.0.id":            "eYake",
		"data.0.playlist_name": "playlist by permalink",
	})
}

func TestPlaylistsEndpointWithAlbumPermalink(t *testing.T) {
	var resp struct {
		Data []dbv1.FullPlaylist
	}

	status, body := testGet(t, "/v1/full/playlists?permalink=/AlbumsByPermalink/album/album-by-permalink", &resp)
	assert.Equal(t, 200, status)

	jsonAssert(t, body, map[string]any{
		"data.0.id":            "ePVXL",
		"data.0.playlist_name": "album by permalink",
	})
}
