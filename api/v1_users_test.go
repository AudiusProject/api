package api

import (
	"encoding/json"
	"testing"

	"api.audius.co/api/dbv1"
	"github.com/RoaringBitmap/roaring"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUserQuery(t *testing.T) {
	app := testAppWithFixtures(t)
	// as anon
	{
		users, err := app.queries.Users(t.Context(), dbv1.GetUsersParams{
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

		// Test that artist_coin_badge includes ticker field
		assert.NotNil(t, user.ArtistCoinBadge)
		var artistCoinBadge map[string]interface{}
		err = json.Unmarshal(user.ArtistCoinBadge, &artistCoinBadge)
		assert.NoError(t, err)
		assert.Equal(t, "TESTCOIN", artistCoinBadge["ticker"])
		assert.Equal(t, "test_mint_address_123", artistCoinBadge["mint"])
		assert.Equal(t, "https://example.com/test-logo.png", artistCoinBadge["logo_uri"])
	}

	// as stereosteve
	{
		users, err := app.queries.Users(t.Context(), dbv1.GetUsersParams{
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
		users, err := app.queries.Users(t.Context(), dbv1.GetUsersParams{
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
		users, err := app.queries.Users(t.Context(), dbv1.GetUsersParams{
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
		users, err := app.queries.Users(t.Context(), dbv1.GetUsersParams{
			MyID: 1,
			Ids:  []int32{3},
		})
		assert.NoError(t, err)
		user := users[0]
		assert.Equal(t, int64(1), user.CurrentUserFolloweeFollowCount)
	}
}

func TestUserQueryUsesSocialSetSnapshots(t *testing.T) {
	app := testAppWithFixtures(t)

	createUserSocialSetsTable(t, app)

	myFollowees := testBitmapBytes(t, 99)
	targetFollowers := testBitmapBytes(t, 99)
	empty := testBitmapBytes(t)

	_, err := app.pool.Exec(t.Context(), `
		INSERT INTO user_social_sets (user_id, followees_bitmap, followers_bitmap)
		VALUES
			(1, $1, $3),
			(3, $3, $2)
		ON CONFLICT (user_id) DO UPDATE SET
			followees_bitmap = EXCLUDED.followees_bitmap,
			followers_bitmap = EXCLUDED.followers_bitmap
	`, myFollowees, targetFollowers, empty)
	require.NoError(t, err)

	users, err := app.queries.Users(t.Context(), dbv1.GetUsersParams{
		MyID: 1,
		Ids:  []int32{3},
	})
	require.NoError(t, err)
	require.Len(t, users, 1)

	assert.False(t, users[0].DoesCurrentUserFollow)
	assert.False(t, users[0].DoesFollowCurrentUser)
	assert.Equal(t, int64(1), users[0].CurrentUserFolloweeFollowCount)
}

func TestUserQuerySocialSetSnapshotsMatchFollows(t *testing.T) {
	app := testAppWithFixtures(t)
	params := dbv1.GetUsersParams{
		MyID: 1,
		Ids:  []int32{1, 2, 3},
	}

	rawUsers, err := app.queries.Users(t.Context(), params)
	require.NoError(t, err)

	createUserSocialSetsTable(t, app)
	for _, userID := range params.Ids {
		require.NoError(t, app.queries.RebuildUserSocialSet(t.Context(), userID))
	}

	snapshotUsers, err := app.queries.Users(t.Context(), params)
	require.NoError(t, err)
	require.Len(t, snapshotUsers, len(rawUsers))

	for i := range rawUsers {
		assert.Equal(t, rawUsers[i].UserID, snapshotUsers[i].UserID)
		assert.Equal(t, rawUsers[i].DoesCurrentUserFollow, snapshotUsers[i].DoesCurrentUserFollow)
		assert.Equal(t, rawUsers[i].DoesCurrentUserSubscribe, snapshotUsers[i].DoesCurrentUserSubscribe)
		assert.Equal(t, rawUsers[i].DoesFollowCurrentUser, snapshotUsers[i].DoesFollowCurrentUser)
		assert.Equal(t, rawUsers[i].CurrentUserFolloweeFollowCount, snapshotUsers[i].CurrentUserFolloweeFollowCount)
	}
}

func createUserSocialSetsTable(t *testing.T, app *ApiServer) {
	t.Helper()

	_, err := app.pool.Exec(t.Context(), `
		CREATE TABLE IF NOT EXISTS user_social_sets (
			user_id integer PRIMARY KEY,
			followees_bitmap bytea NOT NULL DEFAULT '\x'::bytea,
			followers_bitmap bytea NOT NULL DEFAULT '\x'::bytea,
			updated_at timestamp without time zone NOT NULL DEFAULT CURRENT_TIMESTAMP
		)
	`)
	require.NoError(t, err)
}

func testBitmapBytes(t *testing.T, ids ...uint32) []byte {
	t.Helper()
	bitmap := roaring.NewBitmap()
	for _, id := range ids {
		bitmap.Add(id)
	}
	data, err := bitmap.ToBytes()
	require.NoError(t, err)
	return data
}

func TestGetUsers(t *testing.T) {
	app := testAppWithFixtures(t)
	var userResponse struct {
		Data []dbv1.User
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
		Data []dbv1.User
	}

	status, body := testGet(t, app, "/v1/full/users/7eP5n/followers", &userResponse)
	assert.Equal(t, 200, status)

	jsonAssert(t, body, map[string]any{
		"data.0.id":     "ML51L",
		"data.0.handle": "stereosteve",
	})

	assert.Equal(t, "stereosteve", userResponse.Data[0].Handle.String)
}
