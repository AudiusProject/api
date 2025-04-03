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
		DbUrl: "postgres://postgres:example@localhost:21300/test",
	})

	// seed db

	// stupid block fixture
	_, err = app.pool.Exec(ctx, `
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
	insertFixtures("developer_apps", developerAppBaseRow, "testdata/developer_app_fixtures.csv")

	code := m.Run()

	// shutdown()
	os.Exit(code)
}

func TestFixtures(t *testing.T) {
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

func TestGetTracks(t *testing.T) {
	// someone else can only see public tracks
	{
		tracks, err := app.queries.FullTracks(t.Context(), queries.GetTracksParams{
			MyID:    1,
			OwnerID: 2,
		})
		assert.NoError(t, err)
		assert.Len(t, tracks, 1)
		assert.True(t, tracks[0].HasCurrentUserReposted)
	}

	// I can see all my tracks
	{
		tracks, err := app.queries.FullTracks(t.Context(), queries.GetTracksParams{
			MyID:    2,
			OwnerID: 2,
		})
		assert.NoError(t, err)
		assert.Len(t, tracks, 2)
	}

	{
		tracks, err := app.queries.FullTracks(t.Context(), queries.GetTracksParams{
			MyID: 2,
			Ids:  []int32{301},
		})
		assert.NoError(t, err)
		track := tracks[0]
		assert.Equal(t, 135.0, track.DownloadConditions.UsdcPurchase.Price)

	}
}

func TestGetDeveloperAppsQueries(t *testing.T) {
	userId := int32(1)
	developerApps, err := app.queries.GetDeveloperAppsByUser(t.Context(), &userId)
	assert.NoError(t, err)
	assert.Len(t, developerApps, 1)
	assert.Equal(t, "0x7d7b6b7a97d1deefe3a1ccc5a13c48e8f055e0b6", developerApps[0].Address)
}

func TestGetDeveloperApp(t *testing.T) {
	status, body := testGet(t, "/v1/developer_apps/0x7d7b6b7a97d1deefe3a1ccc5a13c48e8f055e0b6")
	assert.Equal(t, 200, status)
	assert.True(t, strings.Contains(string(body), `"user_id":"7eP5n"`))
}

func TestHome(t *testing.T) {
	status, body := testGet(t, "/")
	assert.Equal(t, 200, status)
	assert.Equal(t, "OK", string(body))
}

func TestGetUser(t *testing.T) {
	status, body := testGet(t, "/v1/full/users?id=1")
	assert.Equal(t, 200, status)
	assert.True(t, strings.Contains(string(body), `"handle":"rayjacobson"`))
	assert.True(t, strings.Contains(string(body), `"user_id":1`))
	assert.True(t, strings.Contains(string(body), `"id":"7eP5n"`))
}

func testGet(t *testing.T, path string) (int, []byte) {
	req := httptest.NewRequest("GET", path, nil)
	res, err := app.Test(req, -1)
	assert.NoError(t, err)
	body, _ := io.ReadAll(res.Body)
	return res.StatusCode, body
}
