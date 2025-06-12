package api

import (
	"testing"

	"bridgerton.audius.co/searcher"
	"github.com/test-go/testify/require"
)

func TestSearch(t *testing.T) {
	app := emptyTestApp(t)

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
			},
			{
				"track_id":          1003,
				"owner_id":          1001,
				"title":             "sunny side",
				"genre":             "Jazz",
				"mood":              "Uplifting",
				"stream_conditions": []byte(`{"usdc_purchase": {"price": 135, "splits": [{"user_id": 6, "percentage": 100.0}]}}`),
			},
		},
		"follows": {
			{"follower_user_id": 1001, "followee_user_id": 1002},
		},
	}

	createFixtures(app, fixtures)

	// index data to ES
	searcher.Reindex(app.pool, app.esClient)

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

	{
		status, body := testGet(t, app, "/v1/search/autocomplete?is_verified=true")
		require.Equal(t, 200, status)
		jsonAssert(t, body, map[string]any{
			"data.users.#":        1,
			"data.users.0.handle": "monist",
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

}
