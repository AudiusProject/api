package api

import (
	"testing"

	"bridgerton.audius.co/api/dbv1"
	"bridgerton.audius.co/trashid"
	"github.com/stretchr/testify/assert"
)

func TestPlaylistsEndpoint(t *testing.T) {
	var resp struct {
		Data []dbv1.FullPlaylist
	}

	status, _ := testGet(t, "/v1/full/playlists?id=7eP5n", &resp)
	assert.Equal(t, 200, status)

	pl := resp.Data[0]
	assert.Equal(t, pl.ID, "7eP5n")
	assert.Len(t, pl.Tracks, 2)
	assert.Equal(t, trashid.TrashId(2), pl.Tracks[0].User.ID)
}
