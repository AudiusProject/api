package api

import (
	"context"
	"io"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"bridgerton.audius.co/queries"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
)

var (
	app *ApiServer
)

func checkErr(err error) {
	if err != nil {
		panic(err)
	}
}

func TestMain(m *testing.M) {
	ctx := context.Background()
	var err error

	// create a test db from template
	{
		conn, err := pgx.Connect(ctx, "postgres://postgres:example@localhost:21300/postgres")
		checkErr(err)

		_, err = conn.Exec(ctx, "DROP DATABASE IF EXISTS test")
		checkErr(err)

		_, err = conn.Exec(ctx, "CREATE DATABASE test TEMPLATE postgres")
		checkErr(err)
	}

	app = NewApiServer(Config{
		DBURL: "postgres://postgres:example@localhost:21300/test",
	})

	// seed db

	// stupid block fixture
	_, err = app.conn.Exec(ctx, `
	INSERT INTO public.blocks (
		blockhash,
		parenthash,
		is_current,
		number
	) VALUES (
		'block1',   -- blockhash
		'block0',   -- parenthash
		true,
		101
	);
	`)
	checkErr(err)

	insertFixtures("users", userBaseRow, "testdata/user_fixtures.csv")
	insertFixtures("tracks", trackBaseRow, "testdata/track_fixtures.csv")
	insertFixtures("follows", followBaseRow, "testdata/follow_fixtures.csv")
	insertFixtures("reposts", repostBaseRow, "testdata/repost_fixtures.csv")

	code := m.Run()

	// shutdown()
	os.Exit(code)
}

func TestFixtures(t *testing.T) {
	// as anon
	{
		users, err := app.queries.GetUsers(t.Context(), queries.GetUsersParams{
			Handle: "rayjacobson",
		})
		assert.NoError(t, err)
		user := users[0]
		assert.Equal(t, int32(1), user.UserID)
		assert.Equal(t, "rayjacobson", *user.Handle)
		assert.False(t, user.DoesCurrentUserFollow)
		assert.False(t, user.DoesFollowCurrentUser)
	}

	// as stereosteve
	{
		users, err := app.queries.GetUsers(t.Context(), queries.GetUsersParams{
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
		users, err := app.queries.GetUsers(t.Context(), queries.GetUsersParams{
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
		users, err := app.queries.GetUsers(t.Context(), queries.GetUsersParams{
			MyID: 2,
			Ids:  []int32{1, 2, -1},
		})
		assert.NoError(t, err)
		assert.Len(t, users, 2)
		assert.Equal(t, "rayjacobson", *users[0].Handle)
		assert.Equal(t, "stereosteve", *users[1].Handle)
	}
}

func TestGetTracks(t *testing.T) {
	// someone else can only see public tracks
	{
		tracks, err := app.queries.GetTracks(t.Context(), queries.GetTracksParams{
			MyID:    1,
			OwnerID: 2,
		})
		assert.NoError(t, err)
		assert.Len(t, tracks, 1)
		assert.True(t, tracks[0].HasCurrentUserReposted)
	}

	// I can see all my tracks
	{
		tracks, err := app.queries.GetTracks(t.Context(), queries.GetTracksParams{
			MyID:    2,
			OwnerID: 2,
		})
		assert.NoError(t, err)
		assert.Len(t, tracks, 2)
	}
}

func TestHome(t *testing.T) {
	status, body := testGet(t, "/hello/asdf")
	assert.Equal(t, 200, status)
	assert.Equal(t, "hello asdf", string(body))
}

func TestGetUser(t *testing.T) {
	status, body := testGet(t, "/v2/users/rayjacobson")
	assert.Equal(t, 200, status)
	assert.True(t, strings.Contains(string(body), `"handle":"rayjacobson"`))
	assert.True(t, strings.Contains(string(body), `"user_id":1`))
	assert.True(t, strings.Contains(string(body), `"id":"7eP5n"`))
}

func TestGetBadUser(t *testing.T) {
	status, _ := testGet(t, "/v2/users/badguy")
	assert.Equal(t, 404, status)

	status, _ = testGet(t, "/v2/users/no_exist")
	assert.Equal(t, 404, status)
}

func testGet(t *testing.T, path string) (int, []byte) {
	req := httptest.NewRequest("GET", path, nil)
	res, err := app.Test(req, -1)
	assert.NoError(t, err)
	body, _ := io.ReadAll(res.Body)
	return res.StatusCode, body
}
