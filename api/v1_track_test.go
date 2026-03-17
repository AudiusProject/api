package api

import (
	"context"
	"testing"

	"api.audius.co/api/dbv1"
	"api.audius.co/trashid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetTrack(t *testing.T) {
	app := testAppWithFixtures(t)
	var trackResponse struct {
		Data dbv1.Track
	}

	status, body := testGet(t, app, "/v1/full/tracks/eYJyn", &trackResponse)
	assert.Equal(t, 200, status)

	jsonAssert(t, body, map[string]any{
		"data.id":        "eYJyn",
		"data.title":     "Culca Canyon",
		"data.play_count": 0,
	})
}

func TestGetTrackDownloadCount(t *testing.T) {
	app := testAppWithFixtures(t)
	ctx := context.Background()
	require.NotNil(t, app.writePool, "test requires write pool")

	// Track 200 is "Culca Canyon" (eYJyn). Insert two download rows so download_count is 2.
	_, err := app.writePool.Exec(ctx, `
		INSERT INTO track_downloads (txhash, blocknumber, parent_track_id, track_id, user_id)
		VALUES ('tx-dl-1', 101, 200, 200, 1), ('tx-dl-2', 101, 200, 200, 2)
	`)
	require.NoError(t, err)

	// Single track download_count endpoint
	status, body := testGet(t, app, "/v1/full/tracks/eYJyn/download_count", nil)
	assert.Equal(t, 200, status)
	jsonAssert(t, body, map[string]any{
		"data.id":             "eYJyn",
		"data.download_count": 2,
	})

	// Bulk download_counts endpoint
	status2, body2 := testGet(t, app, "/v1/full/tracks/download_counts?id=eYJyn&id=eYZmn", nil)
	assert.Equal(t, 200, status2)
	jsonAssert(t, body2, map[string]any{
		"data.0.id":             "eYJyn",
		"data.0.download_count": 2,
		"data.1.id":             "eYZmn",
		"data.1.download_count": 0,
	})
}

func TestGetTrackFollowDownloadAcess(t *testing.T) {
	app := testAppWithFixtures(t)
	var trackResponse struct {
		Data dbv1.Track
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
		Data dbv1.Track
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
		Data dbv1.Track
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
		Data dbv1.Track
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
