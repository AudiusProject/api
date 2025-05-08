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
	status, body := testGet(t, "/v1/tracks/trending/underground", &resp)

	assert.Equal(t, 200, status)

	jsonAssert(t, body, map[string]any{
		"data.0.id":    trashid.MustEncodeHashID(300),
		"data.0.genre": "Electronic",

		"data.1.id":    trashid.MustEncodeHashID(202),
		"data.1.genre": "Alternative",

		"data.2.id":    trashid.MustEncodeHashID(200),
		"data.2.genre": "Electronic",
	})

	// These tracks fall outside of underground params (follower / following count)
	for _, track := range resp.Data {
		assert.NotEqual(t, trashid.MustEncodeHashID(501), track.ID, "Track 501 should not be in result set")
		assert.NotEqual(t, trashid.MustEncodeHashID(502), track.ID, "Track 502 should not be in result set")
	}
}
