package api

import (
	"testing"

	"bridgerton.audius.co/api/dbv1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUserQuery(t *testing.T) {
	app := testAppWithFixtures(t)
	// as anon
	{
		users, err := app.queries.FullUsers(t.Context(), dbv1.GetUsersParams{
			Ids: []int32{1},
		})
		assert.NoError(t, err)
		require.Len(t, users, 1)
		user := users[0]
		assert.Equal(t, int32(1), user.UserID)
		// assert.Equal(t, "7eP5n", user.ID)
		assert.Equal(t, "rayjacobson", user.Handle.String)
		assert.False(t, user.DoesCurrentUserFollow)
		assert.False(t, user.DoesFollowCurrentUser)
		assert.Equal(t, int64(0), user.CurrentUserFolloweeFollowCount)
	}

	// as stereosteve
	{
		users, err := app.queries.FullUsers(t.Context(), dbv1.GetUsersParams{
			MyID: 2,
			Ids:  []int32{1},
		})
		assert.NoError(t, err)
		user := users[0]
		assert.Equal(t, "rayjacobson", user.Handle.String)
		assert.True(t, user.DoesCurrentUserFollow)
		assert.True(t, user.DoesFollowCurrentUser)
	}

	// stereosteve views stereosteve
	{
		users, err := app.queries.FullUsers(t.Context(), dbv1.GetUsersParams{
			MyID: 2,
			Ids:  []int32{2},
		})
		assert.NoError(t, err)
		user := users[0]
		assert.Equal(t, "stereosteve", user.Handle.String)
		assert.False(t, user.DoesCurrentUserFollow)
		assert.False(t, user.DoesFollowCurrentUser)
		assert.Equal(t, int64(0), user.CurrentUserFolloweeFollowCount)
	}

	// multiple users
	{
		users, err := app.queries.FullUsers(t.Context(), dbv1.GetUsersParams{
			MyID: 2,
			Ids:  []int32{1, 2, -1},
		})
		assert.NoError(t, err)
		assert.Len(t, users, 2)
		assert.Equal(t, "rayjacobson", users[0].Handle.String)
		assert.Equal(t, "stereosteve", users[1].Handle.String)
	}

	// user 1 follows user 3... user 2 also follows user 3... so user 2 should be counted in CurrentUserFolloweeFollowCount
	{
		users, err := app.queries.FullUsers(t.Context(), dbv1.GetUsersParams{
			MyID: 1,
			Ids:  []int32{3},
		})
		assert.NoError(t, err)
		user := users[0]
		assert.Equal(t, int64(1), user.CurrentUserFolloweeFollowCount)
	}
}

func TestGetUsers(t *testing.T) {
	app := testAppWithFixtures(t)
	var userResponse struct {
		Data []dbv1.FullUser
	}

	status, body := testGet(t, app, "/v1/full/users?id=1", &userResponse)
	assert.Equal(t, 200, status)

	// jsonAssert helps testing the response body
	jsonAssert(t, body, map[string]any{
		"data.0.id":      "7eP5n",
		"data.0.user_id": 1,
		"data.0.handle":  "rayjacobson",
	})

	// but we also unmarshaled into userResponse
	// for structured testing
	assert.Equal(t, "rayjacobson", userResponse.Data[0].Handle.String)

	// this assert won't work:
	// because we have custom json marshal functions
	// assert.Equal(t, userResponse.Data[0].ID, "7eP5n")

	// because it got parsed back to int:
	assert.Equal(t, 1, int(userResponse.Data[0].ID))
}

func TestFollowerEndpoint(t *testing.T) {
	app := testAppWithFixtures(t)
	var userResponse struct {
		Data []dbv1.FullUser
	}

	status, body := testGet(t, app, "/v1/full/users/7eP5n/followers", &userResponse)
	assert.Equal(t, 200, status)

	jsonAssert(t, body, map[string]any{
		"data.0.id":     "ML51L",
		"data.0.handle": "stereosteve",
	})

	assert.Equal(t, "stereosteve", userResponse.Data[0].Handle.String)
}
