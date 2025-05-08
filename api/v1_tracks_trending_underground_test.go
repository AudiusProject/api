package api

import (
	"testing"

	"bridgerton.audius.co/api/dbv1"
	"bridgerton.audius.co/trashid"
	"github.com/stretchr/testify/assert"
)

func TestGetTrendingUnderground(t *testing.T) {
	var resp struct {
		Data []dbv1.FullTrack
	}
	status, _ := testGet(t, "/v1/tracks/trending/underground", &resp)

	assert.Equal(t, 200, status)

	assert.Equal(t, trashid.MustEncodeHashID(300), resp.Data[0].ID)
	assert.Equal(t, "Electronic", resp.Data[0].Genre.String)

	assert.Equal(t, trashid.MustEncodeHashID(202), resp.Data[1].ID)
	assert.Equal(t, "Alternative", resp.Data[1].Genre.String)

	assert.Equal(t, trashid.MustEncodeHashID(200), resp.Data[2].ID)
	assert.Equal(t, "Electronic", resp.Data[2].Genre.String)

	// These tracks fall outside of underground params (follower / following count)
	for _, track := range resp.Data {
		assert.NotEqual(t, trashid.MustEncodeHashID(501), track.ID, "Track 501 should not be in result set")
		assert.NotEqual(t, trashid.MustEncodeHashID(502), track.ID, "Track 502 should not be in result set")
	}
}
