package api

import (
	"testing"

	"bridgerton.audius.co/api/dbv1"
	"bridgerton.audius.co/trashid"
	"github.com/stretchr/testify/assert"
)

func TestGetUsdcPurchase(t *testing.T) {
	app := testAppWithFixtures(t)
	var resp struct {
		Data []dbv1.FullTrack
	}
	status, body := testGet(t, app, "/v1/tracks/usdc-purchase", &resp)

	assert.Equal(t, 200, status)

	jsonAssert(t, body, map[string]any{
		"data.0.id":    trashid.MustEncodeHashID(506),
		"data.0.genre": "Pop",

		"data.1.id":    trashid.MustEncodeHashID(507),
		"data.1.genre": "Jazz",

		"data.2.id":    trashid.MustEncodeHashID(508),
		"data.2.genre": "Classical",
	})
}
