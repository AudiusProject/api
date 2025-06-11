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
				"user_id": 1003,
				"handle":  "monist",
				"name":    "Monist",
			},
		},
		"tracks": {
			{
				"track_id": 1001,
				"owner_id": 1001,
				"title":    "peanut butter jam time",
			},
		},
		"follows": {
			{"follower_user_id": 1001, "followee_user_id": 1002},
		},
	}

	createFixtures(app, fixtures)

	// index data to ES
	searcher.Reindex(app.pool, app.esClient)

	status, body := testGet(t, app, "/v1/search/autocomplete?query=stereo")
	require.Equal(t, 200, status)
	jsonAssert(t, body, map[string]any{
		"data.users.#":        2,
		"data.users.0.handle": "StereoDave",
		"data.users.1.handle": "StereoSteve",
	})

}
