package api

import (
	"testing"

	"bridgerton.audius.co/api/dbv1"
	"bridgerton.audius.co/database"
	"bridgerton.audius.co/trashid"
	"github.com/stretchr/testify/assert"
)

func TestV1UsersRemixers(t *testing.T) {
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
	database.Seed(app.pool, fixtures)

	var userResponse struct {
		Data []dbv1.FullUser
	}

	// Test getting all remixers for user 1
	{
		status, _ := testGet(t, app, "/v1/users/"+trashid.MustEncodeHashID(1)+"/remixers", &userResponse)
		assert.Equal(t, 200, status)
		assert.Len(t, userResponse.Data, 3)
		// Results should be ordered by user_id ASC
		assert.Equal(t, "remixer1", userResponse.Data[0].Handle.String)
		assert.Equal(t, "remixer2", userResponse.Data[1].Handle.String)
		assert.Equal(t, "remixer3", userResponse.Data[2].Handle.String)
	}

	// Test getting remixers filtered by track_id=100
	{
		status, _ := testGet(t, app, "/v1/users/"+trashid.MustEncodeHashID(1)+"/remixers?track_id=100", &userResponse)
		assert.Equal(t, 200, status)
		assert.Len(t, userResponse.Data, 2)
		// Only remixer1 (user 2) and remixer3 (user 4) remixed track 100
		assert.Equal(t, "remixer1", userResponse.Data[0].Handle.String)
		assert.Equal(t, "remixer3", userResponse.Data[1].Handle.String)
	}

	// Test getting remixers filtered by track_id=101
	{
		status, _ := testGet(t, app, "/v1/users/"+trashid.MustEncodeHashID(1)+"/remixers?track_id=101", &userResponse)
		assert.Equal(t, 200, status)
		assert.Len(t, userResponse.Data, 1)
		// Only remixer2 (user 3) remixed track 101
		assert.Equal(t, "remixer2", userResponse.Data[0].Handle.String)
	}

	// Test non-existent track filter
	{
		status, _ := testGet(t, app, "/v1/users/"+trashid.MustEncodeHashID(1)+"/remixers?track_id=999", &userResponse)
		assert.Equal(t, 200, status)
		assert.Len(t, userResponse.Data, 0)
	}
}
