package api

import (
	"testing"

	"bridgerton.audius.co/api/dbv1"
	"bridgerton.audius.co/trashid"
	"github.com/stretchr/testify/assert"
)

func TestGetTrack(t *testing.T) {
	app := testAppWithFixtures(t)
	var trackResponse struct {
		Data dbv1.FullTrack
	}

	status, body := testGet(t, app, "/v1/full/tracks/eYJyn", &trackResponse)
	assert.Equal(t, 200, status)

	jsonAssert(t, body, map[string]any{
		"data.id":    "eYJyn",
		"data.title": "Culca Canyon",
	})
}

func TestGetTrackFollowDownloadAcess(t *testing.T) {
	app := testAppWithFixtures(t)
	var trackResponse struct {
		Data dbv1.FullTrack
	}
	// No access
	_, body1 := testGet(t, app, "/v1/full/tracks/eYRWn", &trackResponse)
	jsonAssert(t, body1, map[string]any{
		"data.title":           "Follow Gated Download",
		"data.access.stream":   true,
		"data.access.download": false,
	})

	// With access
	_, body2 := testGetWithWallet(
		t, app,
		"/v1/full/tracks/eYRWn?user_id=ELKzn",
		"0x4954d18926ba0ed9378938444731be4e622537b2",
		&trackResponse,
	)
	jsonAssert(t, body2, map[string]any{
		"data.title":           "Follow Gated Download",
		"data.access.stream":   true,
		"data.access.download": true,
	})
}

func TestGetTrackTipStreamAccess(t *testing.T) {
	app := testAppWithFixtures(t)
	var trackResponse struct {
		Data dbv1.FullTrack
	}
	// No access
	_, body1 := testGet(t, app, "/v1/full/tracks/L5x7n", &trackResponse)
	jsonAssert(t, body1, map[string]any{
		"data.title":           "Tip Gated Stream",
		"data.access.stream":   false,
		"data.access.download": false,
	})

	// With access
	_, body2 := testGetWithWallet(
		t, app,
		"/v1/full/tracks/L5x7n?user_id=ELKzn",
		"0x4954d18926ba0ed9378938444731be4e622537b2",
		&trackResponse,
	)
	jsonAssert(t, body2, map[string]any{
		"data.title":           "Tip Gated Stream",
		"data.access.stream":   true,
		"data.access.download": true,
	})
}

func TestGetTrackUsdcPurchaseStreamAccess(t *testing.T) {
	app := testAppWithFixtures(t)
	var trackResponse struct {
		Data dbv1.FullTrack
	}
	// No access
	_, body1 := testGet(t, app, "/v1/full/tracks/ebdJL", &trackResponse)
	jsonAssert(t, body1, map[string]any{
		"data.title":           "Pay Gated Stream",
		"data.access.stream":   false,
		"data.access.download": false,
	})

	// With access
	_, body2 := testGetWithWallet(
		t, app,
		"/v1/full/tracks/ebdJL?user_id=1D9On",
		"0x855d28d495ec1b06364bb7a521212753e2190b95",
		&trackResponse,
	)
	jsonAssert(t, body2, map[string]any{
		"data.title":           "Pay Gated Stream",
		"data.access.stream":   true,
		"data.access.download": true,
	})
}

func TestGetTrackUsdcPurchaseSelfAccess(t *testing.T) {
	app := testAppWithFixtures(t)
	var trackResponse struct {
		Data dbv1.FullTrack
	}
	// No access. User 3 is the owner, but has not signed authorization
	status, _ := testGet(
		t, app,
		"/v1/full/tracks/ebdJL?user_id="+trashid.MustEncodeHashID(3),
		&trackResponse,
	)
	assert.Equal(t, 403, status)

	// With access. User 3 is the owner, and has signed authorization
	_, body2 := testGetWithWallet(
		t, app,
		"/v1/full/tracks/ebdJL?user_id="+trashid.MustEncodeHashID(3),
		"0xc3d1d41e6872ffbd15c473d14fc3a9250be5b5e0",
		&trackResponse,
	)
	jsonAssert(t, body2, map[string]any{
		"data.title":           "Pay Gated Stream",
		"data.access.stream":   true,
		"data.access.download": true,
	})
}
