package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"bridgerton.audius.co/api/testdata"
	"bridgerton.audius.co/config"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
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

	app = NewApiServer(config.Config{
		Env:                "test",
		DbUrl:              "postgres://postgres:example@localhost:21300/test",
		DelegatePrivateKey: "0633fddb74e32b3cbc64382e405146319c11a1a52dc96598e557c5dbe2f31468",
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

	insertFixtures("aggregate_user", map[string]any{}, "testdata/aggregate_user_fixtures.csv")
	insertFixtures("users", userBaseRow, "testdata/user_fixtures.csv")
	insertFixtures("tracks", trackBaseRow, "testdata/track_fixtures.csv")
	insertFixtures("playlists", playlistBaseRow, "testdata/playlist_fixtures.csv")
	insertFixtures("follows", followBaseRow, "testdata/follow_fixtures.csv")
	insertFixtures("reposts", repostBaseRow, "testdata/repost_fixtures.csv")
	insertFixtures("developer_apps", developerAppBaseRow, "testdata/developer_app_fixtures.csv")
	insertFixtures("track_trending_scores", trackTrendingScoreBaseRow, "testdata/track_trending_scores_fixtures.csv")
	insertFixtures("associated_wallets", connectedWalletsBaseRow, "testdata/connected_wallets_fixtures.csv")
	insertFixtures("aggregate_user_tips", aggregateUserTipsBaseRow, "testdata/aggregate_user_tips_fixtures.csv")
	insertFixtures("usdc_purchases", usdcPurchaseBaseRow, "testdata/usdc_purchases_fixtures.csv")
	insertFixtures("track_routes", trackRouteBaseRow, "testdata/track_routes_fixtures.csv")
	insertFixtures("playlist_routes", playlistRouteBaseRow, "testdata/playlist_routes_fixtures.csv")
	insertFixtures("grants", grantBaseRow, "testdata/grants_fixtures.csv")
	insertFixtures("comments", commentBaseRow, "testdata/comment_fixtures.csv")
	insertFixtures("comment_threads", map[string]any{}, "testdata/comment_thread_fixtures.csv")
	insertFixtures("muted_users", mutedUserBaseRow, "testdata/muted_users_fixtures.csv")

	// index to es / os

	code := m.Run()

	// shutdown()
	os.Exit(code)
}

func TestHome(t *testing.T) {
	status, body := testGet(t, "/")
	assert.Equal(t, 200, status)
	assert.True(t, strings.Contains(string(body), "uptime"))
}

func Test200UnAuthed(t *testing.T) {
	urls := []string{
		"/v1/full/users?id=7eP5n&id=_some_invalid_hash_id",

		"/v1/full/users/7eP5n",
		"/v1/full/users/7eP5n/followers",
		"/v1/full/users/7eP5n/following",

		"/v1/full/users/7eP5n/library/tracks",
		"/v1/full/users/7eP5n/library/tracks?type=repost&sort_method=plays&sort_direction=asc",
		"/v1/full/users/7eP5n/library/tracks?type=favorite&sort_method=reposts&sort_direction=desc",
		"/v1/full/users/7eP5n/library/tracks?type=purchase",

		"/v1/full/users/7eP5n/library/playlists",
		"/v1/full/users/7eP5n/library/playlists?type=repost&sort_method=plays&sort_direction=asc",
		"/v1/full/users/7eP5n/library/playlists?type=favorite&sort_method=reposts&sort_direction=desc",
		"/v1/full/users/7eP5n/library/albums?type=purchase&sort_method=saves",

		"/v1/full/users/7eP5n/mutuals",
		"/v1/full/users/7eP5n/reposts",
		"/v1/full/users/7eP5n/related",
		"/v1/full/users/7eP5n/supporting",
		"/v1/full/users/7eP5n/supporters",
		"/v1/full/users/7eP5n/tracks",
		"/v1/full/users/7eP5n/feed",
		"/v1/full/users/7eP5n/connected_wallets",

		"/v1/users/7eP5n/tags",

		"/v1/full/users/handle/rayjacobson",
		"/v1/full/users/handle/rayjacobson/tracks",
		"/v1/full/users/handle/rayjacobson/reposts",

		"/v1/full/tracks?id=eYJyn",
		"/v1/full/tracks/eYJyn/reposts",
		"/v1/full/tracks/eYJyn/favorites",
		"/v1/full/tracks/trending",

		"/v1/full/playlists?id=7eP5n",
		"/v1/full/playlists/7eP5n/reposts",
		"/v1/full/playlists/7eP5n/favorites",

		// unclaimed ids
		"/v1/users/unclaimed_id",
		"/v1/tracks/unclaimed_id",
		"/v1/playlists/unclaimed_id",
		"/v1/comments/unclaimed_id",
	}

	for _, u := range urls {
		status, body := testGet(t, u)
		require.Equal(t, 200, status, u+" "+string(body))

		// also test as a user
		if strings.Contains(u, "?") {
			u += "&user_id=7eP5n"
		} else {
			u += "?user_id=7eP5n"
		}

		status, _ = testGet(t, u)
		require.Equal(t, 200, status, u+" "+string(body))
	}
}

func Test200Authed(t *testing.T) {
	urls := []string{
		"/v1/full/users/account/0x7d273271690538cf855e5b3002a0dd8c154bb060",
	}

	for _, u := range urls {
		status, body := testGetWithWallet(t, u, "0x7d273271690538cf855e5b3002a0dd8c154bb060")
		require.Equal(t, 200, status, u+" "+string(body))

		// also test as a user
		if strings.Contains(u, "?") {
			u += "&user_id=7eP5n"
		} else {
			u += "?user_id=7eP5n"
		}

		status, _ = testGetWithWallet(t, u, "0x7d273271690538cf855e5b3002a0dd8c154bb060")
		require.Equal(t, 200, status, u+" "+string(body))
	}

}

func testGet(t *testing.T, path string, dest ...any) (int, []byte) {
	req := httptest.NewRequest("GET", path, nil)
	res, err := app.Test(req, -1)
	assert.NoError(t, err)
	body, _ := io.ReadAll(res.Body)

	if len(dest) > 0 {
		err = json.Unmarshal(body, &dest[0])
		assert.NoError(t, err)
	}

	return res.StatusCode, body
}

func jsonAssert(t *testing.T, body []byte, expectations map[string]string) {
	for path, expectation := range expectations {
		actual := gjson.GetBytes(body, path).String()
		msg := fmt.Sprintf("Expected %s to be %s got %s", path, expectation, actual)
		assert.Equal(t, expectation, actual, msg)
	}
}

// testGetWithWallet makes a GET request with authentication headers for the given wallet address
func testGetWithWallet(t *testing.T, path string, walletAddress string, dest ...any) (int, []byte) {
	req := httptest.NewRequest("GET", path, nil)

	// Add signature headers if wallet address is provided
	if walletAddress != "" {
		sigData := testdata.GetSignatureData(walletAddress)
		req.Header.Set("Encoded-Data-Message", sigData.Message)
		req.Header.Set("Encoded-Data-Signature", sigData.Signature)
	}

	res, err := app.Test(req, -1)
	assert.NoError(t, err)
	body, _ := io.ReadAll(res.Body)

	if len(dest) > 0 {
		err = json.Unmarshal(body, &dest[0])
		assert.NoError(t, err)
	}

	return res.StatusCode, body
}
