package api

import (
	"strings"
	"testing"

	"bridgerton.audius.co/queries"
	"github.com/stretchr/testify/assert"
)

func TestUserQuery(t *testing.T) {
	// as anon
	{
		users, err := app.queries.FullUsers(t.Context(), queries.GetUsersParams{
			Handle: "rayjacobson",
		})
		assert.NoError(t, err)
		user := users[0]
		assert.Equal(t, int32(1), user.UserID)
		assert.Equal(t, "7eP5n", user.ID)
		assert.Equal(t, "rayjacobson", *user.Handle)
		assert.False(t, user.DoesCurrentUserFollow)
		assert.False(t, user.DoesFollowCurrentUser)
	}

	// as stereosteve
	{
		users, err := app.queries.FullUsers(t.Context(), queries.GetUsersParams{
			MyID:   2,
			Handle: "rayjacobson",
		})
		assert.NoError(t, err)
		user := users[0]
		assert.Equal(t, "rayjacobson", *user.Handle)
		assert.True(t, user.DoesCurrentUserFollow)
		assert.True(t, user.DoesFollowCurrentUser)
	}

	// stereosteve views stereosteve
	{
		users, err := app.queries.FullUsers(t.Context(), queries.GetUsersParams{
			MyID: 2,
			Ids:  []int32{2},
		})
		assert.NoError(t, err)
		user := users[0]
		assert.Equal(t, "stereosteve", *user.Handle)
		assert.False(t, user.DoesCurrentUserFollow)
		assert.False(t, user.DoesFollowCurrentUser)
	}

	// multiple users
	{
		users, err := app.queries.FullUsers(t.Context(), queries.GetUsersParams{
			MyID: 2,
			Ids:  []int32{1, 2, -1},
		})
		assert.NoError(t, err)
		assert.Len(t, users, 2)
		assert.Equal(t, "rayjacobson", *users[0].Handle)
		assert.Equal(t, "stereosteve", *users[1].Handle)
	}
}

func TestGetUser(t *testing.T) {
	status, body := testGet(t, "/v1/full/users?id=1")
	assert.Equal(t, 200, status)
	assert.True(t, strings.Contains(string(body), `"handle":"rayjacobson"`))
	assert.True(t, strings.Contains(string(body), `"user_id":1`))
	assert.True(t, strings.Contains(string(body), `"id":"7eP5n"`))
}
