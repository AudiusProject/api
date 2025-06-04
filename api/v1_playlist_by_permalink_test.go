package api

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestV1PlaylistByPermalink(t *testing.T) {
	status, body := testGet(t, "/v1/full/playlists/by_permalink/PlaylistsByPermalink/playlist-by-permalink")
	assert.Equal(t, 200, status)

	jsonAssert(t, body, map[string]any{
		"data.0.id":            "eYake",
		"data.0.playlist_name": "playlist by permalink",
	})
}
