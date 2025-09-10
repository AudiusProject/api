package api

import (
	"testing"

	"api.audius.co/database"
	"api.audius.co/trashid"
	"github.com/stretchr/testify/assert"
)

func TestGetUsersTracksRemixed(t *testing.T) {
	app := emptyTestApp(t)

	fixtures := database.FixtureMap{
		"users": {
			{
				"user_id": 1,
			},
			{
				"user_id": 2,
			},
			{
				"user_id": 3,
			},
			{
				"user_id": 4,
			},
		},
		"tracks": {
			{
				"track_id": 1,
				"title":    "Track 1",
				"owner_id": 1,
			},
			{
				"track_id": 5,
				"title":    "Track 5",
				"owner_id": 1,
			},
			{
				"track_id": 2,
				"title":    "Track 1 remix 1",
				"owner_id": 2,
			},
			{
				"track_id": 3,
				"title":    "Track 1 remix 2",
				"owner_id": 3,
			},
			{
				"track_id": 4,
				"title":    "Track 5 remix 1",
				"owner_id": 4,
			},
		},
		"remixes": {
			{
				"child_track_id":  2,
				"parent_track_id": 1,
			},
			{
				"child_track_id":  3,
				"parent_track_id": 1,
			},
			{
				"child_track_id":  4,
				"parent_track_id": 5,
			},
		},
	}
	database.Seed(app.writePool, fixtures)

	{
		// Test support for handle
		status, body := testGet(t, app, "/v1/users/"+trashid.MustEncodeHashID(1)+"/tracks/remixed")

		assert.Equal(t, 200, status)
		jsonAssert(t, body, map[string]any{
			"data.#":             2,
			"data.0.track_id":    trashid.MustEncodeHashID(1),
			"data.0.title":       "Track 1",
			"data.0.remix_count": 2,
			"data.1.track_id":    trashid.MustEncodeHashID(5),
			"data.1.title":       "Track 5",
			"data.1.remix_count": 1,
		})
	}

	{
		// Test limit/offset
		status, body := testGet(t, app, "/v1/users/"+trashid.MustEncodeHashID(1)+"/tracks/remixed?limit=1&offset=1")

		assert.Equal(t, 200, status)
		jsonAssert(t, body, map[string]any{
			"data.#":             1,
			"data.0.track_id":    trashid.MustEncodeHashID(5),
			"data.0.title":       "Track 5",
			"data.0.remix_count": 1,
		})
	}

}
