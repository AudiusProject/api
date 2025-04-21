package api

import (
	"strings"
	"testing"

	"bridgerton.audius.co/api/dbv1"
	"github.com/stretchr/testify/assert"
)

func TestGetPlaylist(t *testing.T) {
	app := fixturesTestApp(t)

	var playlistResponse struct {
		Data dbv1.FullPlaylist
	}

	status, body := testGet(t, app, "/v1/full/playlists/7eP5n", &playlistResponse)
	assert.Equal(t, 200, status)

	assert.True(t, strings.Contains(string(body), `"playlist_name":"First"`))
	assert.True(t, strings.Contains(string(body), `"id":"7eP5n"`))
}

func TestGetPlaylistFollowDownloadAccess(t *testing.T) {
	app := fixturesTestApp(t)

	var playlistResponse struct {
		Data dbv1.FullPlaylist
	}
	// No access
	_, body1 := testGet(t, app, "/v1/full/playlists/ML51L", &playlistResponse)
	assert.True(t, strings.Contains(string(body1), `"playlist_name":"Follow Gated Stream"`))
	assert.True(t, strings.Contains(string(body1), `"access":{"stream":false,"download":false}`))

	// With access
	_, body2 := testGet(t, app, "/v1/full/playlists/ML51L?user_id=ELKzn", &playlistResponse)
	assert.True(t, strings.Contains(string(body2), `"playlist_name":"Follow Gated Stream"`))
	assert.True(t, strings.Contains(string(body2), `"access":{"stream":true,"download":true}`))
}
