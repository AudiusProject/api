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

func TestMain(m *testing.M) {
	ctx := context.Background()

	checkErr := func(err error) {
		if err != nil {
			panic(err)
		}
	}

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
	tx, err := app.conn.Begin(ctx)
	checkErr(err)

	userFixtures := []struct {
		user_id    int
		handle     string
		deactivate bool
	}{
		{
			user_id: 1,
			handle:  "rayjacobson",
		},
		{
			user_id: 2,
			handle:  "stereosteve",
		},
		{
			user_id:    91,
			handle:     "badguy",
			deactivate: true,
		},
	}
	for _, u := range userFixtures {
		_, err = tx.Exec(ctx, `
		INSERT INTO public.users (
			user_id,
			handle,
			handle_lc,
			is_current,
			is_verified,
			created_at,
			updated_at,
			has_collectibles,
			txhash,
			is_deactivated,
			is_available,
			is_storage_v2,
			allow_ai_attribution
		) VALUES (
			@id,
			@handle,
			@handle_lc,
			true,                 -- is_current
			false,                -- is_verified
			CURRENT_TIMESTAMP,    -- created_at
			CURRENT_TIMESTAMP,    -- updated_at
			false,                -- has_collectibles
			'',                   -- txhash
			@deactivate,                -- is_deactivated
			true,                 -- is_available
			false,                -- is_storage_v2
			false                 -- allow_ai_attribution
		)`, pgx.NamedArgs{
			"id":         u.user_id,
			"handle":     u.handle,
			"handle_lc":  strings.ToLower(u.handle),
			"deactivate": u.deactivate,
		})
		checkErr(err)
	}

	// stupid block fixture
	_, err = tx.Exec(ctx, `
	INSERT INTO public.blocks (
		blockhash,
		parenthash,
		is_current,
		number
	) VALUES (
		'block1',   -- blockhash
		'block0',   -- parenthash
		true,             -- is_current
		101               -- number
	);
	`)
	checkErr(err)

	// follow fixtures
	followFixtures := []struct {
		actorId  int
		targetId int
	}{
		{1, 2},
		{2, 1},
	}
	for _, f := range followFixtures {
		_, err = tx.Exec(ctx, `
		INSERT INTO public.follows (
			blockhash,
			blocknumber,
			follower_user_id,
			followee_user_id,
			is_current,
			is_delete,
			created_at,
			txhash,
			slot
		) VALUES (
			'block1',         -- blockhash
			101,              -- blocknumber
			$1,                -- follower_user_id
			$2,                -- followee_user_id
			true,             -- is_current
			false,            -- is_delete
			CURRENT_TIMESTAMP,-- created_at
			'tx123',          -- txhash
			500               -- slot
		);
		`, f.actorId, f.targetId)
		checkErr(err)
	}

	checkErr(tx.Commit(ctx))

	code := m.Run()

	// shutdown()
	os.Exit(code)
}

func TestFixtures(t *testing.T) {
	// as anon
	{
		user, err := app.queries.GetUserByHandle(t.Context(), queries.GetUserByHandleParams{
			Handle: "rayjacobson",
		})
		assert.NoError(t, err)
		assert.Equal(t, "rayjacobson", *user.Handle)
		assert.False(t, user.DoesCurrentUserFollow)
		assert.False(t, user.DoesFollowCurrentUser)
	}

	// as stereosteve
	{
		user, err := app.queries.GetUserByHandle(t.Context(), queries.GetUserByHandleParams{
			MyID:   2,
			Handle: "rayjacobson",
		})
		assert.NoError(t, err)
		assert.Equal(t, "rayjacobson", *user.Handle)
		assert.True(t, user.DoesCurrentUserFollow)
		assert.True(t, user.DoesFollowCurrentUser)
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
