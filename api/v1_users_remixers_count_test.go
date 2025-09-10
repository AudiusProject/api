package api

import (
	"testing"

	"api.audius.co/database"
	"api.audius.co/trashid"
	"github.com/stretchr/testify/assert"
)

func TestV1UsersRemixersCount(t *testing.T) {
	app := emptyTestApp(t)
	fixtures := database.FixtureMap{
		"tracks": []map[string]any{
			{
				"track_id": 100,
				"owner_id": 1,
				"title":    "Original Track 1",
			},
			{
				"track_id": 101,
				"owner_id": 1,
				"title":    "Original Track 2",
			},
			{
				"track_id": 200,
				"owner_id": 2,
				"title":    "Remix of Track 1",
			},
			{
				"track_id": 300,
				"owner_id": 3,
				"title":    "Remix of Track 2",
			},
			{
				"track_id": 400,
				"owner_id": 4,
				"title":    "Another Remix of Track 1",
			},
		},
		"users": []map[string]any{
			{
				"user_id":   1,
				"handle":    "rayjacobson",
				"handle_lc": "rayjacobson",
			},
			{
				"user_id":   2,
				"handle":    "remixer1",
				"handle_lc": "remixer1",
				"wallet":    "0x1234567890abcdef",
			},
			{
				"user_id":   3,
				"handle":    "remixer2",
				"handle_lc": "remixer2",
				"wallet":    "0xc3d1d41e6872ffbd15c473d14fc3a9250be5b5e0",
			},
			{
				"user_id":   4,
				"handle":    "remixer3",
				"handle_lc": "remixer3",
				"wallet":    "0x9876543210fedcba",
			},
		},
		"remixes": []map[string]any{
			{
				"parent_track_id": 100,
				"child_track_id":  200,
			},
			{
				"parent_track_id": 101,
				"child_track_id":  300,
			},
			{
				"parent_track_id": 100,
				"child_track_id":  400,
			},
		},
	}
	database.Seed(app.pool.Replicas[0], fixtures)

	// Test count for all remixers for user 1
	{
		status, body := testGet(t, app, "/v1/users/"+trashid.MustEncodeHashID(1)+"/remixers/count")
		assert.Equal(t, 200, status)
		jsonAssert(t, body, map[string]any{
			"data": 3,
		})
	}

	// Test count for remixers filtered by track_id=100
	{
		status, body := testGet(t, app, "/v1/users/"+trashid.MustEncodeHashID(1)+"/remixers/count?track_id=100")
		assert.Equal(t, 200, status)
		jsonAssert(t, body, map[string]any{
			"data": 2,
		})
	}

	// Test count for non-existent track filter
	{
		status, body := testGet(t, app, "/v1/users/"+trashid.MustEncodeHashID(1)+"/remixers/count?track_id=999")
		assert.Equal(t, 200, status)
		jsonAssert(t, body, map[string]any{
			"data": 0,
		})
	}
}
