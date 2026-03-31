package api

import (
	"crypto/ed25519"
	"io"
	"net/http/httptest"
	"net/url"
	"testing"

	"api.audius.co/database"
	"api.audius.co/trashid"
	"github.com/mr-tron/base58"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testFanClubFeedMint = "FanClubMintTest11111111111111111111111"
func testFanClubFeedBaseUsers() []map[string]any 
	return []map[string]any{
		{
			"user_id":   1,
			"handle":    "fcartist",
			"handle_lc": "fcartist",
			"name":      "FC Artist",
			"wallet":    "0x7d273271690538cf855e5b3002a0dd8c154bb060",
		},
		{
			"user_id":   2,
			"handle":    "fcfan",
			"handle_lc": "fcfan",
			"name":      "FC Fan",
			"wallet":    "0xc3d1d41e6872ffbd15c473d14fc3a9250be5b5e0",
		},
	}
}

func testFanClubFeedURL(viewerUserID int, extraQuery string) string {
	v := url.Values{}
	v.Set("mint", testFanClubFeedMint)
	if viewerUserID > 0 {
		v.Set("user_id", trashid.MustEncodeHashID(viewerUserID))
	}
	if extraQuery != "" {
		more, err := url.ParseQuery(extraQuery)
		if err != nil {
			panic(err)
		}
		for k, vals := range more {
			for _, val := range vals {
				v.Add(k, val)
			}
		}
	}
	return "/v1/fan_club/feed?" + v.Encode()
}

func testGetWithSolanaHeaders(t *testing.T, app *ApiServer, path string, walletPub58, message string, sig []byte) (int, []byte) {
	t.Helper()
	req := httptest.NewRequest("GET", path, nil)
	req.Header.Set("X-Solana-Wallet", walletPub58)
	req.Header.Set("X-Solana-Message", message)
	req.Header.Set("X-Solana-Signature", base58.Encode(sig))
	res, err := app.Test(req, -1)
	require.NoError(t, err)
	body, err := io.ReadAll(res.Body)
	require.NoError(t, err)
	return res.StatusCode, body
}

func TestFanClubFeed_MissingMint(t *testing.T) {
	app := emptyTestApp(t)
	status, _ := testGet(t, app, "/v1/fan_club/feed")
	assert.Equal(t, 400, status)
}

func TestFanClubFeed_UnknownMint(t *testing.T) {
	app := emptyTestApp(t)
	database.Seed(app.pool.Replicas[0], database.FixtureMap{
		"users": testFanClubFeedBaseUsers(),
	})
	status, _ := testGet(t, app, "/v1/fan_club/feed?mint=UnknownMintZZZZZZZZZZZZZZZZZZZZZZZZZZZ")
	assert.Equal(t, 404, status)
}

func TestFanClubFeed_UnauthedStillReturnsFeed(t *testing.T) {
	app := emptyTestApp(t)
	database.Seed(app.pool.Replicas[0], database.FixtureMap{
		"users": testFanClubFeedBaseUsers(),
		"artist_coins": {
			{
				"user_id":  1,
				"mint":     testFanClubFeedMint,
				"decimals": 6,
				"ticker":   "FCT",
			},
		},
	})
	status, _ := testGet(t, app, testFanClubFeedURL(0, ""))
	assert.Equal(t, 200, status)
}

func TestFanClubFeed_ArtistSeesOnlyCoinGatedTrack(t *testing.T) {
	app := emptyTestApp(t)
	database.Seed(app.pool.Replicas[0], database.FixtureMap{
		"users": testFanClubFeedBaseUsers(),
		"artist_coins": {
			{
				"user_id":  1,
				"mint":     testFanClubFeedMint,
				"decimals": 6,
				"ticker":   "FCT",
			},
		},
		"tracks": {
			{
				"track_id":        701,
				"owner_id":        1,
				"title":           "Coin gated for feed",
				"is_stream_gated": true,
				"stream_conditions": map[string]any{
					"token_gate": map[string]any{
						"token_mint":   testFanClubFeedMint,
						"token_amount": 1,
					},
				},
				"created_at": "2020-01-02 00:00:00",
			},
			{
				"track_id":        702,
				"owner_id":        1,
				"title":           "Public not gated",
				"is_stream_gated": false,
				"created_at":      "2020-01-03 00:00:00",
			},
			{
				"track_id":        703,
				"owner_id":        1,
				"title":           "Gated other mint",
				"is_stream_gated": true,
				"stream_conditions": map[string]any{
					"token_gate": map[string]any{
						"token_mint":   "OtherMintZZZZZZZZZZZZZZZZZZZZZZZZZZ",
						"token_amount": 1,
					},
				},
				"created_at": "2020-01-04 00:00:00",
			},
		},
	})

	path := testFanClubFeedURL(1, "")
	status, body := testGetWithWallet(t, app, path, "0x7d273271690538cf855e5b3002a0dd8c154bb060")
	require.Equal(t, 200, status, string(body))

	enc701, err := trashid.EncodeHashId(701)
	require.NoError(t, err)

	jsonAssert(t, body, map[string]any{
		"data.#":             1,
		"data.0.item_type":   "track",
		"data.0.track.title": "Coin gated for feed",
		"data.0.track.id":    enc701,
		"data.1.item_type":   "",
		"related.tracks.#":   1,
	})
}

func TestFanClubFeed_FanWithBalanceSeesFeed(t *testing.T) {
	app := emptyTestApp(t)
	database.Seed(app.pool.Replicas[0], database.FixtureMap{
		"users": testFanClubFeedBaseUsers(),
		"artist_coins": {
			{
				"user_id":  1,
				"mint":     testFanClubFeedMint,
				"decimals": 6,
				"ticker":   "FCT",
			},
		},
		"sol_user_balances": {
			{
				"user_id": 2,
				"mint":    testFanClubFeedMint,
				"balance": int64(1_000_000),
			},
		},
		"tracks": {
			{
				"track_id":        801,
				"owner_id":        1,
				"title":           "Gated track",
				"is_stream_gated": true,
				"stream_conditions": map[string]any{
					"token_gate": map[string]any{
						"token_mint":   testFanClubFeedMint,
						"token_amount": 1,
					},
				},
				"created_at": "2020-01-01 00:00:00",
			},
		},
	})

	path := testFanClubFeedURL(2, "")
	status, body := testGetWithWallet(t, app, path, "0xc3d1d41e6872ffbd15c473d14fc3a9250be5b5e0")
	require.Equal(t, 200, status, string(body))

	enc801, err := trashid.EncodeHashId(801)
	require.NoError(t, err)

	jsonAssert(t, body, map[string]any{
		"data.#":             1,
		"data.0.item_type":   "track",
		"data.0.track.id":    enc801,
		"data.0.track.title": "Gated track",
	})
}

func TestFanClubFeed_IncludesTextPostWhenCommentRowPresent(t *testing.T) {
	app := emptyTestApp(t)

	commentRow := map[string]any{
		"comment_id":  910,
		"user_id":     2,
		"entity_id":   1,
		"entity_type": "FanClub",
		"text":        "hello fan club",
		"created_at":  "2020-06-01 00:00:00",
	}

	database.Seed(app.pool.Replicas[0], database.FixtureMap{
		"users": testFanClubFeedBaseUsers(),
		"artist_coins": {
			{
				"user_id":  1,
				"mint":     testFanClubFeedMint,
				"decimals": 6,
				"ticker":   "FCT",
			},
		},
		"sol_user_balances": {
			{
				"user_id": 2,
				"mint":    testFanClubFeedMint,
				"balance": int64(1_000_000),
			},
		},
		"tracks": {
			{
				"track_id":        901,
				"owner_id":        1,
				"title":           "Gated",
				"is_stream_gated": true,
				"stream_conditions": map[string]any{
					"token_gate": map[string]any{
						"token_mint":   testFanClubFeedMint,
						"token_amount": 1,
					},
				},
				"created_at": "2020-01-01 00:00:00",
			},
		},
		"comments": []map[string]any{commentRow},
	})

	path := testFanClubFeedURL(2, "sort_method=newest")
	status, body := testGetWithWallet(t, app, path, "0xc3d1d41e6872ffbd15c473d14fc3a9250be5b5e0")
	if status != 200 {
		t.Fatalf("expected 200 got %d: %s", status, string(body))
	}

	enc910, err := trashid.EncodeHashId(910)
	require.NoError(t, err)

	jsonAssert(t, body, map[string]any{
		"data.#":                 2,
		"data.0.item_type":       "text_post",
		"data.0.comment.message": "hello fan club",
		"data.0.comment.id":      enc910,
		"data.1.item_type":       "track",
		"data.1.track.title":     "Gated",
	})
}

func TestFanClubFeed_FanWithoutBalanceGetsFeedWithRedactedText(t *testing.T) {
	app := emptyTestApp(t)
	database.Seed(app.pool.Replicas[0], database.FixtureMap{
		"users": testFanClubFeedBaseUsers(),
		"artist_coins": {
			{
				"user_id":  1,
				"mint":     testFanClubFeedMint,
				"decimals": 6,
				"ticker":   "FCT",
			},
		},
		"sol_user_balances": {
			{
				"user_id": 2,
				"mint":    testFanClubFeedMint,
				"balance": int64(1),
			},
		},
		"tracks": {
			{
				"track_id":        902,
				"owner_id":        1,
				"title":           "Gated",
				"is_stream_gated": true,
				"stream_conditions": map[string]any{
					"token_gate": map[string]any{
						"token_mint":   testFanClubFeedMint,
						"token_amount": 1,
					},
				},
				"created_at": "2020-01-01 00:00:00",
			},
		},
		"comments": []map[string]any{
			{
				"comment_id":  920,
				"user_id":     2,
				"entity_id":   1,
				"entity_type": "FanClub",
				"text":        "secret post",
				"created_at":  "2020-06-01 00:00:00",
			},
		},
	})

	path := testFanClubFeedURL(2, "sort_method=newest")
	status, body := testGetWithWallet(t, app, path, "0xc3d1d41e6872ffbd15c473d14fc3a9250be5b5e0")
	require.Equal(t, 200, status, string(body))

	jsonAssert(t, body, map[string]any{
		"data.#":           2,
		"data.0.item_type": "text_post",
		"data.1.item_type": "track",
	})
	jsonAssert(t, body, map[string]any{
		"data.0.comment.message": nil,
	})
}

func TestFanClubFeed_ExternalSolanaWalletRevealsTextPost(t *testing.T) {
	app := emptyTestApp(t)
	pub, priv, err := ed25519.GenerateKey(nil)
	require.NoError(t, err)
	wallet58 := base58.Encode(pub)
	msg := "Audius fan club"
	sig := ed25519.Sign(priv, []byte(msg))

	database.Seed(app.pool.Replicas[0], database.FixtureMap{
		"users": testFanClubFeedBaseUsers(),
		"artist_coins": {
			{
				"user_id":  1,
				"mint":     testFanClubFeedMint,
				"decimals": 6,
				"ticker":   "FCT",
			},
		},
		"sol_token_account_balances": {
			{
				"mint":    testFanClubFeedMint,
				"account": "fcfeedta1",
				"owner":   wallet58,
				"balance": int64(1_000_000),
			},
		},
		"comments": []map[string]any{
			{
				"comment_id":  930,
				"user_id":     2,
				"entity_id":   1,
				"entity_type": "FanClub",
				"text":        "holders only",
				"created_at":  "2020-06-01 00:00:00",
			},
		},
	})

	path := testFanClubFeedURL(0, "")
	status, body := testGetWithSolanaHeaders(t, app, path, wallet58, msg, sig)
	require.Equal(t, 200, status, string(body))

	enc930, err := trashid.EncodeHashId(930)
	require.NoError(t, err)
	jsonAssert(t, body, map[string]any{
		"data.#":                 1,
		"data.0.item_type":       "text_post",
		"data.0.comment.message": "holders only",
		"data.0.comment.id":      enc930,
	})
}
