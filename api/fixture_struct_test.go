package api

import (
	"testing"

	"bridgerton.audius.co/api/dbv1"
	"github.com/stretchr/testify/assert"
)

type FixtureMap map[string][]map[string]any

func TestFixtureStruct(t *testing.T) {
	app := emptyTestApp(t)

	fixtures := FixtureMap{
		"users": {
			{
				"user_id": 1001,
				"handle":  "test 123",
			},
			{
				"user_id": 1002,
				"handle":  "test 999",
			},
		},
		"tracks": {
			{
				"track_id": 1001,
				"owner_id": 1001,
				"title":    "my jam",
			},
		},
		"follows": {
			{"follower_user_id": 1001, "followee_user_id": 1002},
		},
	}

	createFixtures(app, fixtures)

	users, err := app.queries.FullUsers(t.Context(), dbv1.GetUsersParams{
		Ids:  []int32{1001, 1002},
		MyID: 1001,
	})
	assert.NoError(t, err)
	assert.Equal(t, "test 123", users[0].Handle.String)
	assert.True(t, users[1].DoesCurrentUserFollow)

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
