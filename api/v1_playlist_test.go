package api

import (
	"testing"

	"bridgerton.audius.co/api/dbv1"
	"bridgerton.audius.co/trashid"
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
	_, body2 := testGetWithWallet(
		t,
		"/v1/full/playlists/ML51L?user_id=ELKzn",
		"0x4954d18926ba0ed9378938444731be4e622537b2",
		&playlistResponse,
	)
	jsonAssert(t, body2, map[string]string{
		"data.0.playlist_name": "Follow Gated Stream",
		"data.0.access":        `{"stream":true,"download":true}`,
	})
}

func TestGetPlaylistUsdcPurchaseStreamAccess(t *testing.T) {
	var playlistResponse struct {
		Data []dbv1.FullPlaylist
	}
	// No access
	_, body1 := testGet(t, "/v1/full/playlists/ELKzn", &playlistResponse)
	jsonAssert(t, body1, map[string]string{
		"data.0.playlist_name": "Purchase Gated Stream",
		"data.0.access":        `{"stream":false,"download":false}`,
	})

	// With access
	_, body2 := testGetWithWallet(
		t,
		"/v1/full/playlists/ELKzn?user_id=1D9On",
		"0x855d28d495ec1b06364bb7a521212753e2190b95",
		&playlistResponse,
	)
	jsonAssert(t, body2, map[string]string{
		"data.0.playlist_name": "Purchase Gated Stream",
		"data.0.access":        `{"stream":true,"download":true}`,
	})
}

func TestGetPlaylistUsdcPurchaseSelfAccess(t *testing.T) {
	var playlistResponse struct {
		Data []dbv1.FullPlaylist
	}
	// No access. User 3 is the owner, but has not signed authorization
	status, _ := testGet(
		t,
		"/v1/full/playlists/ELKzn?user_id="+trashid.MustEncodeHashID(3),
		&playlistResponse,
	)
	assert.Equal(t, 403, status)

	// With access. User 3 is the owner, and has signed authorization
	_, body2 := testGetWithWallet(
		t,
		"/v1/full/playlists/ELKzn?user_id="+trashid.MustEncodeHashID(3),
		"0xc3d1d41e6872ffbd15c473d14fc3a9250be5b5e0",
		&playlistResponse,
	)
	jsonAssert(t, body2, map[string]string{
		"data.0.playlist_name": "Purchase Gated Stream",
		"data.0.access":        `{"stream":true,"download":true}`,
	})
}
