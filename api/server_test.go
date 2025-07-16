package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http/httptest"
	"strings"
	"testing"

	"bridgerton.audius.co/api/testdata"
	"bridgerton.audius.co/config"
	"bridgerton.audius.co/database"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"

	_ "github.com/jackc/pgx/v5/stdlib"
)

func checkErr(err error) {
	if err != nil {
		panic(err)
	}
}

func emptyTestApp(t *testing.T) *ApiServer {
	pool := database.CreateTestDatabase(t)

	app := NewApiServer(config.Config{
		Env:                "test",
		ReadDbUrl:          pool.Config().ConnString(),
		EsUrl:              "http://localhost:21400",
		DelegatePrivateKey: "0633fddb74e32b3cbc64382e405146319c11a1a52dc96598e557c5dbe2f31468",
		SolanaConfig:       config.SolanaConfig{RpcProviders: []string{""}},
	})

	t.Cleanup(func() {
		app.pool.Close()
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

	database.SeedTable(app.pool, "aggregate_plays", testdata.AggregatePlays)
	database.SeedTable(app.pool, "aggregate_track", testdata.AggregateTrack)
	database.SeedTable(app.pool, "aggregate_user", testdata.AggregateUser)
	database.SeedTable(app.pool, "aggregate_user_tips", testdata.AggregateUserTips)
	database.SeedTable(app.pool, "audio_transactions_history", testdata.AudioTransactionsHistory)
	database.SeedTable(app.pool, "challenges", testdata.Challenges)
	database.SeedTable(app.pool, "challenge_listen_streak", testdata.ChallengeListenStreak)
	database.SeedTable(app.pool, "comments", testdata.Comment)
	database.SeedTable(app.pool, "comment_threads", testdata.CommentThread)
	database.SeedTable(app.pool, "associated_wallets", testdata.ConnectedWallets)
	database.SeedTable(app.pool, "developer_apps", testdata.DeveloperApps)
	database.SeedTable(app.pool, "events", testdata.Events)
	database.SeedTable(app.pool, "follows", testdata.Follows)
	database.SeedTable(app.pool, "grants", testdata.Grants)
	database.SeedTable(app.pool, "playlists", testdata.Playlists)
	database.SeedTable(app.pool, "playlist_routes", testdata.PlaylistRoutesFixtures)
	database.SeedTable(app.pool, "playlist_trending_scores", testdata.PlaylistTrendingScores)
	database.SeedTable(app.pool, "reposts", testdata.RepostFixtures)
	database.SeedTable(app.pool, "saves", testdata.SaveFixtures)
	database.SeedTable(app.pool, "tracks", testdata.TrackFixtures)
	database.SeedTable(app.pool, "track_trending_scores", testdata.TrackTrendingScoresFixtures)
	database.SeedTable(app.pool, "track_routes", testdata.TrackRoutesFixtures)
	database.SeedTable(app.pool, "usdc_purchases", testdata.UsdcPurchasesFixtures)
	database.SeedTable(app.pool, "usdc_transactions_history", testdata.UsdcTransactionsHistoryFixtures)
	database.SeedTable(app.pool, "user_bank_accounts", testdata.UserBankAccountsFixtures)
	database.SeedTable(app.pool, "user_challenges", testdata.UserChallengesFixtures)
	database.SeedTable(app.pool, "usdc_user_bank_accounts", testdata.UserBankAccountsFixtures)
	database.SeedTable(app.pool, "users", testdata.UserFixtures)
	database.SeedTable(app.pool, "user_listening_history", testdata.UserListeningHistoryFixtures)

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

func jsonAssert(t *testing.T, body []byte, expectations map[string]any) bool {
	success := true
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
		case nil:
			actual = gjson.GetBytes(body, path).Value()
		default:
			t.Errorf("unsupported type for expectation: %T", v)
		}
		msg := fmt.Sprintf("Expected %s to be %v got %v", path, expectation, actual)
		if !assert.Equal(t, expectation, actual, msg) {
			success = false
		}
	}
	return success
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
