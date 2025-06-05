package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/rand/v2"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"

	"bridgerton.audius.co/api/testdata"
	"bridgerton.audius.co/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"

	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
)

func checkErr(err error) {
	if err != nil {
		panic(err)
	}
}

var testMutex = sync.Mutex{}
var testPoolForCreatingChildDatabases *pgxpool.Pool

func TestMain(m *testing.M) {
	ctx := context.Background()
	var err error

	testPoolForCreatingChildDatabases, err = pgxpool.New(ctx, "postgres://postgres:example@localhost:21300/test01")
	checkErr(err)

	// run tests
	code := m.Run()

	os.Exit(code)
}

func emptyTestApp(t *testing.T) *ApiServer {
	t.Parallel()
	t.Helper()

	dbName := fmt.Sprintf("testdb_%d", rand.Int())
	ctx := context.Background()

	// create a test db from template
	testMutex.Lock()
	_, err := testPoolForCreatingChildDatabases.Exec(ctx, "CREATE DATABASE "+dbName+" TEMPLATE test01")
	testMutex.Unlock()
	require.NoError(t, err)

	app := NewApiServer(config.Config{
		Env:                "test",
		DbUrl:              "postgres://postgres:example@localhost:21300/" + dbName,
		DelegatePrivateKey: "0633fddb74e32b3cbc64382e405146319c11a1a52dc96598e557c5dbe2f31468",
		SolanaConfig:       config.SolanaConfig{RpcProviders: []string{""}},
	})

	t.Cleanup(func() {
		app.pool.Close()
		testMutex.Lock()
		_, err := testPoolForCreatingChildDatabases.Exec(ctx, "DROP DATABASE IF EXISTS test")
		testMutex.Unlock()
		require.NoError(t, err)
	})

	return app
}

func testAppWithFixtures(t *testing.T) *ApiServer {
	ctx := context.Background()
	app := emptyTestApp(t)

	// seed db
	// stupid block fixture
	_, err := app.pool.Exec(ctx, `
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

	insertFixturesFromArray(app, "aggregate_plays", map[string]any{}, testdata.AggregatePlays)
	insertFixturesFromArray(app, "aggregate_track", map[string]any{}, testdata.AggregateTrack)
	insertFixturesFromArray(app, "aggregate_user", map[string]any{}, testdata.AggregateUser)
	insertFixturesFromArray(app, "aggregate_user_tips", aggregateUserTipsBaseRow, testdata.AggregateUserTips)
	insertFixturesFromArray(app, "audio_transactions_history", audioTransactionBaseRow, testdata.AudioTransactionsHistory)
	insertFixturesFromArray(app, "challenges", map[string]any{}, testdata.Challenges)
	insertFixturesFromArray(app, "challenge_listen_streak", listenStreakBaseRow, testdata.ChallengeListenStreak)
	insertFixturesFromArray(app, "comments", commentBaseRow, testdata.Comment)
	insertFixturesFromArray(app, "comment_threads", map[string]any{}, testdata.CommentThread)
	insertFixturesFromArray(app, "associated_wallets", connectedWalletsBaseRow, testdata.ConnectedWallets)
	insertFixturesFromArray(app, "developer_apps", developerAppBaseRow, testdata.DeveloperApps)
	insertFixturesFromArray(app, "events", eventBaseRow, testdata.Events)
	insertFixturesFromArray(app, "follows", followBaseRow, testdata.Follows)
	insertFixturesFromArray(app, "grants", grantBaseRow, testdata.Grants)
	insertFixturesFromArray(app, "playlists", playlistBaseRow, testdata.Playlists)
	insertFixturesFromArray(app, "playlist_routes", playlistRouteBaseRow, testdata.PlaylistRoutesFixtures)
	insertFixturesFromArray(app, "playlist_trending_scores", playlistTrendingScoreBaseRow, testdata.PlaylistTrendingScores)
	insertFixturesFromArray(app, "reposts", repostBaseRow, testdata.RepostFixtures)
	insertFixturesFromArray(app, "tracks", trackBaseRow, testdata.TrackFixtures)
	insertFixturesFromArray(app, "track_trending_scores", trackTrendingScoreBaseRow, testdata.TrackTrendingScoresFixtures)
	insertFixturesFromArray(app, "track_routes", trackRouteBaseRow, testdata.TrackRoutesFixtures)
	insertFixturesFromArray(app, "usdc_purchases", usdcPurchaseBaseRow, testdata.UsdcPurchasesFixtures)
	insertFixturesFromArray(app, "usdc_transactions_history", usdcTransactionBaseRow, testdata.UsdcTransactionsHistoryFixtures)
	insertFixturesFromArray(app, "user_bank_accounts", userBankBaseRow, testdata.UserBankAccountsFixtures)
	insertFixturesFromArray(app, "user_challenges", userChallengeBaseRow, testdata.UserChallengesFixtures)
	insertFixturesFromArray(app, "usdc_user_bank_accounts", usdcUserBankBaseRow, testdata.UserBankAccountsFixtures)
	insertFixturesFromArray(app, "users", userBaseRow, testdata.UserFixtures)
	insertFixturesFromArray(app, "user_listening_history", map[string]any{}, testdata.UserListeningHistoryFixtures)

	return app

}

func TestHome(t *testing.T) {
	app := emptyTestApp(t)
	status, body := testGet(t, app, "/")
	assert.Equal(t, 200, status)
	assert.True(t, strings.Contains(string(body), "uptime"))
}

func Test200UnAuthed(t *testing.T) {
	app := testAppWithFixtures(t)

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

		"/v1/full/users/7eP5n/managers",
		"/v1/full/users/7eP5n/managed_users",
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
		"/v1/full/playlists/trending",
		// unclaimed ids
		"/v1/users/unclaimed_id",
		"/v1/tracks/unclaimed_id",
		"/v1/playlists/unclaimed_id",
		"/v1/comments/unclaimed_id",
	}

	for _, u := range urls {
		status, body := testGet(t, app, u)
		require.Equal(t, 200, status, u+" "+string(body))

		// also test as a user
		if strings.Contains(u, "?") {
			u += "&user_id=7eP5n"
		} else {
			u += "?user_id=7eP5n"
		}

		status, _ = testGetWithWallet(t, app, u, "0x7d273271690538cf855e5b3002a0dd8c154bb060")
		require.Equal(t, 200, status, u+" "+string(body))
	}
}

func Test200Authed(t *testing.T) {
	app := testAppWithFixtures(t)

	urls := []string{
		"/v1/full/users/account/0x7d273271690538cf855e5b3002a0dd8c154bb060",
	}

	for _, u := range urls {
		status, body := testGetWithWallet(t, app, u, "0x7d273271690538cf855e5b3002a0dd8c154bb060")
		require.Equal(t, 200, status, u+" "+string(body))

		// also test as a user
		if strings.Contains(u, "?") {
			u += "&user_id=7eP5n"
		} else {
			u += "?user_id=7eP5n"
		}

		status, _ = testGetWithWallet(t, app, u, "0x7d273271690538cf855e5b3002a0dd8c154bb060")
		require.Equal(t, 200, status, u+" "+string(body))
	}

}

func testGet(t *testing.T, app *ApiServer, path string, dest ...any) (int, []byte) {
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

func jsonAssert(t *testing.T, body []byte, expectations map[string]any) {
	for path, expectation := range expectations {
		var actual any
		switch v := expectation.(type) {
		case string:
			actual = gjson.GetBytes(body, path).String()
		case bool:
			actual = gjson.GetBytes(body, path).Bool()
		case float64:
			actual = gjson.GetBytes(body, path).Float()
		case int:
			actual = int(gjson.GetBytes(body, path).Int())
		default:
			t.Errorf("unsupported type for expectation: %T", v)
		}
		msg := fmt.Sprintf("Expected %s to be %v got %v", path, expectation, actual)
		assert.Equal(t, expectation, actual, msg)
	}
}

// testGetWithWallet makes a GET request with authentication headers for the given wallet address
func testGetWithWallet(t *testing.T, app *ApiServer, path string, walletAddress string, dest ...any) (int, []byte) {
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
