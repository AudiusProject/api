package api

import (
	"testing"

	"api.audius.co/api/dbv1"
	"api.audius.co/database"
	"api.audius.co/trashid"
	"github.com/stretchr/testify/assert"
)

func TestGetPlaylist(t *testing.T) {
	app := testAppWithFixtures(t)
	var playlistResponse struct {
		Data []dbv1.Playlist
	}

	status, body := testGet(t, app, "/v1/full/playlists/7eP5n", &playlistResponse)
	assert.Equal(t, 200, status)

	jsonAssert(t, body, map[string]any{
		"data.0.id":               "7eP5n",
		"data.0.playlist_name":    "First",
		"data.0.total_play_count": 0,
	})
}

func TestGetPlaylistFollowDownloadAccess(t *testing.T) {
	app := testAppWithFixtures(t)
	var playlistResponse struct {
		Data []dbv1.Playlist
	}
	// No access
	_, body1 := testGet(t, app, "/v1/full/playlists/ML51L", &playlistResponse)
	jsonAssert(t, body1, map[string]any{
		"data.0.playlist_name": "Follow Gated Stream",
		"data.0.access":        `{"stream":false,"download":false}`,
	})

	// With access
	_, body2 := testGetWithWallet(
		t, app,
		"/v1/full/playlists/ML51L?user_id=ELKzn",
		"0x4954d18926ba0ed9378938444731be4e622537b2",
		&playlistResponse,
	)
	jsonAssert(t, body2, map[string]any{
		"data.0.playlist_name": "Follow Gated Stream",
		"data.0.access":        `{"stream":true,"download":true}`,
	})
}

func TestGetPlaylistUsdcPurchaseStreamAccess(t *testing.T) {
	app := testAppWithFixtures(t)
	var playlistResponse struct {
		Data []dbv1.Playlist
	}
	// No access
	_, body1 := testGet(t, app, "/v1/full/playlists/ELKzn", &playlistResponse)
	jsonAssert(t, body1, map[string]any{
		"data.0.playlist_name": "Purchase Gated Stream",
		"data.0.access":        `{"stream":false,"download":false}`,
	})

	// With access
	_, body2 := testGetWithWallet(
		t, app,
		"/v1/full/playlists/ELKzn?user_id=1D9On",
		"0x855d28d495ec1b06364bb7a521212753e2190b95",
		&playlistResponse,
	)
	jsonAssert(t, body2, map[string]any{
		"data.0.playlist_name": "Purchase Gated Stream",
		"data.0.access":        `{"stream":true,"download":true}`,
	})
}

func TestGetPlaylistUsdcPurchaseSelfAccess(t *testing.T) {
	app := testAppWithFixtures(t)
	var playlistResponse struct {
		Data []dbv1.Playlist
	}
	// No access. User 3 is the owner, but has not signed authorization
	status, _ := testGet(
		t, app,
		"/v1/full/playlists/ELKzn?user_id="+trashid.MustEncodeHashID(3),
		&playlistResponse,
	)
	assert.Equal(t, 403, status)

	// With access. User 3 is the owner, and has signed authorization
	_, body2 := testGetWithWallet(
		t, app,
		"/v1/full/playlists/ELKzn?user_id="+trashid.MustEncodeHashID(3),
		"0xc3d1d41e6872ffbd15c473d14fc3a9250be5b5e0",
		&playlistResponse,
	)
	jsonAssert(t, body2, map[string]any{
		"data.0.playlist_name": "Purchase Gated Stream",
		"data.0.access":        `{"stream":true,"download":true}`,
	})
}

func TestGetPlaylistIncludesUnlistedTracks(t *testing.T) {
	app := emptyTestApp(t)

	fixtures := database.FixtureMap{
		"users": []map[string]any{
			{
				"user_id": 1,
				"handle":  "user1",
				"name":    "User 1",
			},
		},
		"tracks": []map[string]any{
			{
				"track_id": 1,
				"owner_id": 1,
				"title":    "Listed Track",
			},
			{
				"track_id":    2,
				"owner_id":    1,
				"title":       "Unlisted Track",
				"is_unlisted": true,
			},
		},
		"playlists": []map[string]any{
			{
				"playlist_id":       1,
				"playlist_owner_id": 1,
				"playlist_contents": map[string]any{
					"track_ids": []map[string]any{
						{"track": 1, "time": 1, "metadata_time": 1},
						{"track": 2, "time": 2, "metadata_time": 2},
					},
				},
			},
		},
	}
	database.Seed(app.pool.Replicas[0], fixtures)

	playlistId := trashid.MustEncodeHashID(1)

	var playlistResponse struct {
		Data []dbv1.Playlist
	}
	status, body := testGet(t, app, "/v1/full/playlists/"+playlistId, &playlistResponse)
	assert.Equal(t, 200, status)

	// Both listed and unlisted tracks should appear in the hydrated tracks array
	jsonAssert(t, body, map[string]any{
		"data.0.tracks.#":    2,
		"data.0.tracks.0.id": trashid.MustEncodeHashID(1),
		"data.0.tracks.1.id": trashid.MustEncodeHashID(2),
	})
}
