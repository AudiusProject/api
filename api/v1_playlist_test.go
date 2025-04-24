package api

import (
	"testing"

	"bridgerton.audius.co/api/dbv1"
	"github.com/stretchr/testify/assert"
)

func TestGetPlaylist(t *testing.T) {
	var playlistResponse struct {
		Data []dbv1.FullPlaylist
	}

	status, body := testGet(t, "/v1/full/playlists/7eP5n", &playlistResponse)
	assert.Equal(t, 200, status)

	jsonAssert(t, body, map[string]string{
		"data.0.id":            "7eP5n",
		"data.0.playlist_name": "First",
	})
}

func TestGetPlaylistFollowDownloadAccess(t *testing.T) {
	var playlistResponse struct {
		Data []dbv1.FullPlaylist
	}
	// No access
	_, body1 := testGet(t, "/v1/full/playlists/ML51L", &playlistResponse)
	jsonAssert(t, body1, map[string]string{
		"data.0.playlist_name": "Follow Gated Stream",
		"data.0.access":        `{"stream":false,"download":false}`,
	})

	// With access
	_, body2 := testGet(t, "/v1/full/playlists/ML51L?user_id=ELKzn", &playlistResponse)
	jsonAssert(t, body2, map[string]string{
		"data.0.playlist_name": "Follow Gated Stream",
		"data.0.access":        `{"stream":true,"download":true}`,
	})
}

func TestGetPlaylistUsdcPurchaseStreamAccess(t *testing.T) {
	var playlistResponse struct {
		Data dbv1.FullPlaylist
	}
	// No access
	_, body1 := testGet(t, "/v1/full/playlists/ELKzn", &playlistResponse)
	jsonAssert(t, body1, map[string]string{
		"data.playlist_name":   "Purchase Gated Stream",
		"data.access.stream":   "false",
		"data.access.download": "false",
	})

	// With access
	_, body2 := testGet(t, "/v1/full/playlists/ELKzn?user_id=1D9On", &playlistResponse)
	jsonAssert(t, body2, map[string]string{
		"data.playlist_name":   "Purchase Gated Stream",
		"data.access.stream":   "true",
		"data.access.download": "true",
	})
}
