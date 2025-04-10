package api

import (
	"strings"
	"testing"

	"bridgerton.audius.co/api/dbv1"
	"github.com/stretchr/testify/assert"
)

func TestGetPlaylist(t *testing.T) {
	var playlistResponse struct {
		Data dbv1.FullPlaylist
	}

	status, body := testGet(t, "/v1/full/playlists/7eP5n", &playlistResponse)
	assert.Equal(t, 200, status)

	assert.True(t, strings.Contains(string(body), `"playlist_name":"First"`))
	assert.True(t, strings.Contains(string(body), `"id":"7eP5n"`))
}
