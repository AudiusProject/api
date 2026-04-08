package api

import (
	"testing"

	"api.audius.co/database"
	"api.audius.co/trashid"
	"github.com/stretchr/testify/assert"
)

func TestV1PlaylistTracks(t *testing.T) {
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
				"title":    "Track 1",
			},
			{
				"track_id": 2,
				"owner_id": 1,
				"title":    "Track 2",
			},
			{
				"track_id": 3,
				"owner_id": 1,
				"title":    "Track 3",
			},
			{
				"track_id": 4,
				"owner_id": 1,
				"title":    "Track 4",
			},
			{
				"track_id":        5,
				"owner_id":        1,
				"title":           "Track 5",
				"is_stream_gated": true,
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
						{"track": 3, "time": 3, "metadata_time": 3},
						{"track": 4, "time": 4, "metadata_time": 4},
						{"track": 5, "time": 5, "metadata_time": 5},
					},
				},
			},
		},
	}
	database.Seed(app.pool.Replicas[0], fixtures)

	playlistId := trashid.MustEncodeHashID(1)

	{
		status, body := testGet(t, app, "/v1/playlists/"+playlistId+"/tracks", nil)
		assert.Equal(t, 200, status)
		jsonAssert(t, body, map[string]any{
			"data.#":    4,
			"data.0.id": trashid.MustEncodeHashID(1),
			"data.1.id": trashid.MustEncodeHashID(2),
			"data.2.id": trashid.MustEncodeHashID(3),
			"data.3.id": trashid.MustEncodeHashID(4),
		})
	}

	{
		status, body := testGet(t, app, "/v1/playlists/"+playlistId+"/tracks?exclude_gated=false", nil)
		assert.Equal(t, 200, status)
		jsonAssert(t, body, map[string]any{
			"data.#":    5,
			"data.4.id": trashid.MustEncodeHashID(5),
		})
	}
}

func TestV1PlaylistTracksIncludesUnlisted(t *testing.T) {
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

	// GET /v1/playlists/:id/tracks should include unlisted tracks
	{
		status, body := testGet(t, app, "/v1/playlists/"+playlistId+"/tracks", nil)
		assert.Equal(t, 200, status)
		jsonAssert(t, body, map[string]any{
			"data.#":    2,
			"data.0.id": trashid.MustEncodeHashID(1),
			"data.1.id": trashid.MustEncodeHashID(2),
		})
	}

	// Also works with exclude_gated=false
	{
		status, body := testGet(t, app, "/v1/playlists/"+playlistId+"/tracks?exclude_gated=false", nil)
		assert.Equal(t, 200, status)
		jsonAssert(t, body, map[string]any{
			"data.#": 2,
		})
	}
}
