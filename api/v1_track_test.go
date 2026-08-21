package api

import (
	"testing"

	"api.audius.co/api/dbv1"
	"api.audius.co/database"
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
		"data.id":         "eYJyn",
		"data.title":      "Culca Canyon",
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
		"data.id":                         "eYJyn",
		"data.has_current_user_reposted":  false,
		"data.has_current_user_saved":     false,
		"data.followee_reposts.0.user_id": trashid.MustEncodeHashID(1),
	})

	_, body = testGet(t, app, "/v1/full/tracks/eYZmn?user_id=ML51L", &resp)
	jsonAssert(t, body, map[string]any{
		"data.id":                           "eYZmn",
		"data.has_current_user_reposted":    false,
		"data.has_current_user_saved":       false,
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

// A track whose owner is no longer active - the artist deactivated their own
// account, or the account was delisted by the trusted notifier - must not carry
// signed content-node URLs in its response. The stream and download endpoints
// already reject these, but the media links in the track response bypass those
// endpoints entirely: the cid is real, so the signed URL serves the full audio
// straight from the content node to anyone who reads the response.
func TestGetTrack_NonStreamableOmitsMediaLinks(t *testing.T) {
	for _, tc := range []struct {
		name  string
		track map[string]any
		user  map[string]any
	}{
		{
			name: "deactivated owner",
			track: map[string]any{
				"track_id": 1, "owner_id": 1, "title": "Deactivated Owner",
				"track_cid": "QmTrackCid", "orig_file_cid": "QmOrigCid",
				"preview_cid": "QmPreviewCid", "is_downloadable": true,
			},
			user: map[string]any{"user_id": 1, "handle": "testuser1", "is_deactivated": true},
		},
		{
			name: "deleted track",
			track: map[string]any{
				"track_id": 1, "owner_id": 1, "title": "Deleted",
				"track_cid": "QmTrackCid", "orig_file_cid": "QmOrigCid",
				"preview_cid": "QmPreviewCid", "is_downloadable": true,
				"is_delete": true,
			},
			user: map[string]any{"user_id": 1, "handle": "testuser1"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			app := emptyTestApp(t)
			database.Seed(app.pool.Replicas[0], database.FixtureMap{
				"tracks": []map[string]any{tc.track},
				"users":  []map[string]any{tc.user},
			})

			var resp struct{ Data dbv1.Track }
			status, _ := testGet(t, app, "/v1/full/tracks/"+trashid.MustEncodeHashID(1), &resp)
			assert.Equal(t, 200, status)

			assert.False(t, resp.Data.IsStreamable)
			assert.Nil(t, resp.Data.Stream)
			assert.Nil(t, resp.Data.Download)
			assert.Nil(t, resp.Data.Preview)
		})
	}
}

// The guard above is scoped to non-streamable tracks: an ordinary track with an
// active owner must still get its signed media links.
func TestGetTrack_StreamableKeepsMediaLinks(t *testing.T) {
	app := emptyTestApp(t)
	database.Seed(app.pool.Replicas[0], database.FixtureMap{
		"tracks": []map[string]any{
			{
				"track_id": 1, "owner_id": 1, "title": "Active Owner",
				"track_cid": "QmTrackCid", "orig_file_cid": "QmOrigCid",
				"preview_cid": "QmPreviewCid", "is_downloadable": true,
			},
		},
		"users": []map[string]any{{"user_id": 1, "handle": "testuser1"}},
	})

	var resp struct{ Data dbv1.Track }
	status, _ := testGet(t, app, "/v1/full/tracks/"+trashid.MustEncodeHashID(1), &resp)
	assert.Equal(t, 200, status)

	assert.True(t, resp.Data.IsStreamable)
	assert.NotNil(t, resp.Data.Stream)
	assert.NotNil(t, resp.Data.Download)
	assert.NotNil(t, resp.Data.Preview)
	assert.Contains(t, resp.Data.Stream.Url, "signature=")
}
