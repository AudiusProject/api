package api

import (
	"testing"

	"bridgerton.audius.co/searcher"
	"github.com/test-go/testify/require"
)

func TestSearch(t *testing.T) {
	app := emptyTestApp(t)
	app.skipAuthCheck = true

	fixtures := FixtureMap{
		"users": {
			{
				"user_id": 1001,
				"handle":  "StereoSteve",
				"name":    "Stereo Steve",
			},
			{
				"user_id": 1002,
				"handle":  "StereoDave",
				"name":    "Stereo Dave",
			},
			{
				"user_id":     1003,
				"handle":      "monist",
				"name":        "Monist",
				"is_verified": true,
			},
		},
		"tracks": {
			{
				"track_id":    1001,
				"owner_id":    1001,
				"title":       "peanut butter jam time",
				"genre":       "Trap",
				"bpm":         88,
				"mood":        "Defiant",
				"musical_key": "A minor",
			},
			{
				"track_id":        1002,
				"owner_id":        1001,
				"title":           "mouse trap",
				"genre":           "Trap",
				"bpm":             99,
				"mood":            "Uplifting",
				"is_downloadable": true,
				"musical_key":     "B minor",
				"tags":            "Tag1,Tag2,Tag3",
			},
			{
				"track_id":          1003,
				"owner_id":          1001,
				"title":             "sunny side",
				"genre":             "Jazz",
				"mood":              "Uplifting",
				"stream_conditions": []byte(`{"usdc_purchase": {"price": 135, "splits": [{"user_id": 6, "percentage": 100.0}]}}`),
			},
			{
				"track_id":  1004,
				"owner_id":  1001,
				"title":     "hide deleted",
				"is_delete": true,
			},
			{
				"track_id":    1005,
				"owner_id":    1001,
				"title":       "hide private",
				"is_unlisted": true,
			},
			{
				"track_id":     1006,
				"owner_id":     1001,
				"title":        "hide unavailable",
				"is_available": false,
			},
			{
				"track_id": 1007,
				"owner_id": 1003,
				"title":    "circular thoughts",
			},
		},
		"playlists": {
			{
				"playlist_id":       9001,
				"playlist_owner_id": 1001,
				"playlist_name":     "Old and Busted",
			},
			{
				"playlist_id":       9002,
				"playlist_owner_id": 1001,
				"playlist_name":     "My Old Album",
				"is_album":          true,
			},
			{
				"playlist_id":       9003,
				"playlist_owner_id": 1001,
				"playlist_name":     "Hot and New",
			},
		},
		"playlist_tracks": {
			{
				"playlist_id": 9003,
				"track_id":    1007,
			},
		},
		"follows": {
			{"follower_user_id": 1001, "followee_user_id": 1002},
		},
		"reposts": {
			{
				"repost_item_id": 1001,
				"user_id":        1001,
				"repost_type":    "track",
				"created_at":     parseTime(t, "2024-01-03"),
			},
			{
				"repost_item_id": 1001,
				"user_id":        1002,
				"repost_type":    "track",
				"created_at":     parseTime(t, "2024-01-03"),
			},
			{
				"repost_item_id": 1003,
				"user_id":        1003,
				"repost_type":    "track",
				"created_at":     parseTime(t, "2024-01-03"),
			},
		},
	}

	createFixtures(app, fixtures)

	// index data to ES
	searcher.Reindex(app.pool, app.esClient, true)

	// users:
	{
		status, body := testGet(t, app, "/v1/search/autocomplete?query=stereo")
		require.Equal(t, 200, status)
		jsonAssert(t, body, map[string]any{
			"data.users.#":        2,
			"data.users.0.handle": "StereoDave",
			"data.users.1.handle": "StereoSteve",
		})
	}

	// users: prefix match
	{
		status, body := testGet(t, app, "/v1/search/autocomplete?query=ster")
		require.Equal(t, 200, status)
		jsonAssert(t, body, map[string]any{
			"data.users.#":        2,
			"data.users.0.handle": "StereoDave",
			"data.users.1.handle": "StereoSteve",
		})
	}

	{
		status, body := testGet(t, app, "/v1/users/search?query=stereo")
		require.Equal(t, 200, status)
		jsonAssert(t, body, map[string]any{
			"data.#":        2,
			"data.0.handle": "StereoDave",
			"data.1.handle": "StereoSteve",
		})
	}

	{
		status, body := testGet(t, app, "/v1/search/autocomplete?is_verified=true")
		require.Equal(t, 200, status)
		jsonAssert(t, body, map[string]any{
			"data.users.#":        1,
			"data.users.0.handle": "monist",
		})
	}

	// tracks: default rank is by repost count
	{
		status, body := testGet(t, app, "/v1/search/autocomplete")
		require.Equal(t, 200, status)
		jsonAssert(t, body, map[string]any{
			"data.tracks.0.title":        "peanut butter jam time",
			"data.tracks.0.repost_count": 2,
		})
	}

	// but if you pass a user_id and have reposted a track...
	// your history will rank it higher
	{
		status, body := testGet(t, app, "/v1/search/autocomplete?user_id=1003")
		require.Equal(t, 200, status)
		jsonAssert(t, body, map[string]any{
			"data.tracks.0.title":        "sunny side",
			"data.tracks.0.repost_count": 1,
		})
	}

	// tracks: filter by genre + mood + bpm
	{
		status, body := testGet(t, app, "/v1/search/autocomplete?genre=trap")
		require.Equal(t, 200, status)
		jsonAssert(t, body, map[string]any{
			"data.tracks.#": 2,
		})
	}

	// tracks: only verified
	{
		status, body := testGet(t, app, "/v1/search/autocomplete?only_verified=true")
		require.Equal(t, 200, status)
		jsonAssert(t, body, map[string]any{
			"data.tracks.0.title": "circular thoughts",
		})
	}

	// can search artist + track title
	{
		status, body := testGet(t, app, "/v1/search/autocomplete?query=stereo+sun")
		require.Equal(t, 200, status)
		jsonAssert(t, body, map[string]any{
			// "data.tracks.#":       3,
			"data.tracks.0.title": "sunny side",
		})
	}

	// can search tags
	{
		status, body := testGet(t, app, "/v1/search/autocomplete?query=Tag2")
		require.Equal(t, 200, status)
		jsonAssert(t, body, map[string]any{
			"data.tracks.#":       1,
			"data.tracks.0.title": "mouse trap",
		})
	}

	// doesn't show deleted or unlisted tracks
	{
		status, body := testGet(t, app, "/v1/search/autocomplete?query=hide")
		require.Equal(t, 200, status)
		jsonAssert(t, body, map[string]any{
			"data.tracks.#": 0,
		})
	}

	// can search artist handle
	{
		status, body := testGet(t, app, "/v1/search/autocomplete?query=stereosteve")
		require.Equal(t, 200, status)
		jsonAssert(t, body, map[string]any{
			"data.tracks.#":             3,
			"data.tracks.0.user.handle": "StereoSteve",
		})
	}

	// can search artist name
	if false {
		status, body := testGet(t, app, "/v1/search/autocomplete?query=stereo+steve")
		require.Equal(t, 200, status)
		jsonAssert(t, body, map[string]any{
			"data.tracks.#":       1,
			"data.tracks.0.title": "sunny side",
		})
	}

	{
		status, body := testGet(t, app, "/v1/search/autocomplete?genre=trap&bpm_min=90")
		require.Equal(t, 200, status)
		jsonAssert(t, body, map[string]any{
			"data.tracks.#":       1,
			"data.tracks.0.title": "mouse trap",
		})
	}

	{
		status, body := testGet(t, app, "/v1/search/autocomplete?genre=trap&bpm_max=90")
		require.Equal(t, 200, status)
		jsonAssert(t, body, map[string]any{
			"data.tracks.#":       1,
			"data.tracks.0.title": "peanut butter jam time",
		})
	}

	{
		status, body := testGet(t, app, "/v1/search/autocomplete?genre=trap&genre=jazz")
		require.Equal(t, 200, status)
		jsonAssert(t, body, map[string]any{
			"data.tracks.#": 3,
		})
	}

	{
		status, body := testGet(t, app, "/v1/search/autocomplete?genre=trap&genre=jazz&mood=uplifting")
		require.Equal(t, 200, status)
		jsonAssert(t, body, map[string]any{
			"data.tracks.#": 2,
		})
	}

	{
		status, body := testGet(t, app, "/v1/search/autocomplete?is_downloadable=true")
		require.Equal(t, 200, status)
		jsonAssert(t, body, map[string]any{
			"data.tracks.#":       1,
			"data.tracks.0.title": "mouse trap",
		})
	}

	{
		status, body := testGet(t, app, "/v1/search/autocomplete?key=A+minor")
		require.Equal(t, 200, status)
		jsonAssert(t, body, map[string]any{
			"data.tracks.#":       1,
			"data.tracks.0.title": "peanut butter jam time",
		})
	}

	{
		status, body := testGet(t, app, "/v1/search/autocomplete?key=A+minor&key=B+minor")
		require.Equal(t, 200, status)
		jsonAssert(t, body, map[string]any{
			"data.tracks.#": 2,
		})
	}

	{
		status, body := testGet(t, app, "/v1/search/autocomplete?is_purchaseable=true")
		require.Equal(t, 200, status)
		jsonAssert(t, body, map[string]any{
			"data.tracks.#":       1,
			"data.tracks.0.title": "sunny side",
		})
	}

	//
	// Playlists
	//

	{
		status, body := testGet(t, app, "/v1/search/autocomplete?query=old")
		require.Equal(t, 200, status)
		jsonAssert(t, body, map[string]any{
			"data.playlists.#":               1,
			"data.playlists.0.playlist_name": "Old and Busted",
			"data.albums.#":                  1,
			"data.albums.0.playlist_name":    "My Old Album",
		})
	}

	{
		status, body := testGet(t, app, "/v1/search/autocomplete?only_verified=true")
		require.Equal(t, 200, status)
		jsonAssert(t, body, map[string]any{
			"data.playlists.0.playlist_name": "Hot and New",
		})
	}

	//
	// tag search
	//

	// users:
	{
		status, body := testGet(t, app, "/v1/search/tags?query=Tag2")
		require.Equal(t, 200, status)
		jsonAssert(t, body, map[string]any{
			"data.users.#":        1,
			"data.users.0.handle": "StereoSteve",
			"data.tracks.#":       1,
			"data.tracks.0.title": "mouse trap",
		})
	}
}
