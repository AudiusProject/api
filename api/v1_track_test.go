package api

import (
	"testing"

	"api.audius.co/api/dbv1"
	"api.audius.co/trashid"
	"github.com/stretchr/testify/assert"
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

// Regression coverage for the my_follows CTE in get_tracks.sql. The CTE
// powers `has_current_user_*`, `followee_reposts`, and `followee_favorites`
// and was changed to MATERIALIZED for performance — these assertions guard
// against future refactors silently breaking the personalization shape.
func TestGetTrackPersonalization(t *testing.T) {
	app := testAppWithFixtures(t)
	app.skipAuthCheck = true

	var resp struct{ Data dbv1.Track }

	// Track 200 (eYJyn) is reposted by user 1, who is followed by user 2 (ML51L).
	// Track 100 (eYZmn) is saved by user 1.
	// Querying as user 2 should populate followee_* with user 1, and current-user
	// repost/save flags should be false.
	_, body := testGet(t, app, "/v1/full/tracks/eYJyn?user_id=ML51L", &resp)
	jsonAssert(t, body, map[string]any{
		"data.id":                        "eYJyn",
		"data.has_current_user_reposted": false,
		"data.has_current_user_saved":    false,
		"data.followee_reposts.0.user_id": trashid.MustEncodeHashID(1),
	})

	_, body = testGet(t, app, "/v1/full/tracks/eYZmn?user_id=ML51L", &resp)
	jsonAssert(t, body, map[string]any{
		"data.id":                          "eYZmn",
		"data.has_current_user_reposted":   false,
		"data.has_current_user_saved":      false,
		"data.followee_favorites.0.user_id": trashid.MustEncodeHashID(1),
	})

	// Querying as user 1 themselves: their own repost/save shows on the flags;
	// followee_* should not include themselves.
	_, body = testGet(t, app, "/v1/full/tracks/eYJyn?user_id="+trashid.MustEncodeHashID(1), &resp)
	jsonAssert(t, body, map[string]any{
		"data.has_current_user_reposted": true,
	})
	_, body = testGet(t, app, "/v1/full/tracks/eYZmn?user_id="+trashid.MustEncodeHashID(1), &resp)
	jsonAssert(t, body, map[string]any{
		"data.has_current_user_saved": true,
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
