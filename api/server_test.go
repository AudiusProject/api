package api

import (
	"context"
	"io"
	"net/http/httptest"
	"os"
	"testing"

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

func TestHome(t *testing.T) {
	status, body := testGet(t, "/")
	assert.Equal(t, 200, status)
	assert.Equal(t, "OK", string(body))
}

func testGet(t *testing.T, path string) (int, []byte) {
	req := httptest.NewRequest("GET", path, nil)
	res, err := app.Test(req, -1)
	assert.NoError(t, err)
	body, _ := io.ReadAll(res.Body)
	return res.StatusCode, body
}
