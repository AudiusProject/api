package api

import (
	"strings"
	"testing"

	"bridgerton.audius.co/api/dbv1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUserQuery(t *testing.T) {
	// as anon
	{
		users, err := app.queries.FullUsers(t.Context(), dbv1.GetUsersParams{
			Ids: []int32{1},
		})
		assert.NoError(t, err)
		require.Len(t, users, 1)
		user := users[0]
		assert.Equal(t, int32(1), user.UserID)
		assert.Equal(t, "7eP5n", user.ID)
		assert.Equal(t, "rayjacobson", user.Handle.String)
		assert.False(t, user.DoesCurrentUserFollow)
		assert.False(t, user.DoesFollowCurrentUser)
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
}

func TestGetUsers(t *testing.T) {
	var userResponse struct {
		Data []dbv1.FullUser
	}

	status, body := testGet(t, "/v1/full/users?id=1", &userResponse)
	assert.Equal(t, 200, status)

	// body is response json
	assert.True(t, strings.Contains(string(body), `"handle":"rayjacobson"`))
	assert.True(t, strings.Contains(string(body), `"user_id":1`))
	assert.True(t, strings.Contains(string(body), `"id":"7eP5n"`))

	// but we also unmarshaled into userResponse
	// for structured testing
	assert.Equal(t, userResponse.Data[0].ID, "7eP5n")
}

func TestFollowerEndpoint(t *testing.T) {
	var userResponse struct {
		Data []dbv1.FullUser
	}

	status, _ := testGet(t, "/v1/full/users/7eP5n/followers", &userResponse)
	assert.Equal(t, 200, status)
	assert.Equal(t, userResponse.Data[0].ID, "ML51L")
}
