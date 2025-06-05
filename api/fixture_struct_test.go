package api

import (
	"testing"

	"bridgerton.audius.co/api/dbv1"
	"github.com/stretchr/testify/assert"
)

func TestFixtureStruct(t *testing.T) {
	app := emptyTestApp(t)

	fixtures := FixtureSet{
		users: []map[string]any{
			{
				"user_id": 1001,
				"handle":  "test 123",
			},
			{
				"user_id": 1002,
				"handle":  "test 999",
			},
		},
		tracks: []map[string]any{
			{
				"track_id": 1001,
				"owner_id": 1001,
				"title":    "my jam",
			},
		},
	}

	createFixtures(app, fixtures)

	users, err := app.queries.FullUsers(t.Context(), dbv1.GetUsersParams{
		Ids: []int32{1001, 1002},
	})
	assert.NoError(t, err)
	assert.Equal(t, "test 123", users[0].Handle.String)

	tracks, err := app.queries.FullTracks(t.Context(), dbv1.FullTracksParams{
		GetTracksParams: dbv1.GetTracksParams{
			Ids:  []int32{1001},
			MyID: int32(0),
		},
	})
	assert.NoError(t, err)
	assert.Equal(t, "test 123", tracks[0].User.Handle.String)
	assert.Equal(t, "my jam", tracks[0].Title.String)
}
