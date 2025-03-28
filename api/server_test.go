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

	// track fixtures
	trackFixtures := []struct {
		id          int
		owner_id    int
		title       string
		is_unlisted bool
	}{
		{
			id:       200,
			owner_id: 2,
			title:    "Culca Canyon",
		},
		{
			id:          201,
			owner_id:    2,
			title:       "Turkey Time DEMO",
			is_unlisted: true,
		},
	}
	for _, t := range trackFixtures {
		_, err := tx.Exec(ctx, `
		INSERT INTO public.tracks (
			blockhash,
			track_id,
			is_current,
			is_delete,
			owner_id,
			title,
			genre,
			mood,
			created_at,
			updated_at,
			txhash,
			is_unlisted,
			is_available,
			track_segments,
			is_scheduled_release,
			is_downloadable,
			is_original_available,
			playlists_containing_track,
			playlists_previously_containing_track,
			audio_analysis_error_count,
			is_owned_by_user
		) VALUES (
			'block_abc123',                    -- blockhash
			@track_id,                              -- track_id
			true,                              -- is_current
			false,                             -- is_delete
			@owner_id,                                 -- owner_id
			@title,                  -- title
			'Electronic',                      -- genre
			'Energetic',                       -- mood
			CURRENT_TIMESTAMP,                 -- created_at
			CURRENT_TIMESTAMP,                 -- updated_at
			'tx_123abc',                       -- txhash
			@is_unlisted,                             -- is_unlisted
			true,                              -- is_available
			'[]'::jsonb,                       -- track_segments
			false,                             -- is_scheduled_release
			false,                             -- is_downloadable
			false,                             -- is_original_available
			'{}',                              -- playlists_containing_track
			jsonb_build_object(),              -- playlists_previously_containing_track
			0,                                 -- audio_analysis_error_count
			false                              -- is_owned_by_user
		);`, pgx.NamedArgs{
			"track_id":    t.id,
			"owner_id":    t.owner_id,
			"title":       t.title,
			"is_unlisted": t.is_unlisted,
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

	// reposts
	repostFixtures := []struct {
		user_id        int
		repost_type    string
		repost_item_id int
	}{
		{1, "track", 200},
	}
	for _, r := range repostFixtures {
		_, err := tx.Exec(ctx, `
		INSERT INTO public.reposts (
			blockhash,
			blocknumber,
			user_id,
			repost_item_id,
			repost_type,
			is_current,
			is_delete,
			created_at,
			txhash,
			slot,
			is_repost_of_repost
		) VALUES (
			'block_abc123',       -- blockhash
			101,                  -- blocknumber
			@user_id,                   -- user_id
			@repost_item_id,                 -- repost_item_id (e.g., track ID or playlist ID)
			@repost_type,              -- repost_type (must be a valid value from reposttype enum)
			true,                 -- is_current
			false,                -- is_delete
			CURRENT_TIMESTAMP,    -- created_at
			'tx_456def',          -- txhash
			500,                  -- slot
			false                 -- is_repost_of_repost
		);
		`, pgx.NamedArgs{
			"user_id":        r.user_id,
			"repost_type":    r.repost_type,
			"repost_item_id": r.repost_item_id,
		})
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
