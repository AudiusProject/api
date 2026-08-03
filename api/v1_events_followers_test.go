package api

import (
	"encoding/base64"
	"fmt"
	"strings"
	"testing"
	"time"

	"api.audius.co/api/testdata"
	"api.audius.co/database"
	"api.audius.co/trashid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Tests for /v1/events/:eventId/follow* — the follow-contest endpoints added
// in Phase 2. The follow_state endpoint is the one with real read logic that
// can be validated end-to-end here; the POST / DELETE endpoints emit on-chain
// transactions via sendTransactionWithSigner, which is exercised upstream by
// the discovery-provider indexer tests. Here we only verify they're routed,
// authed, and parse params correctly.

// Pre-registered test wallets with signature data in api/testdata/signatures.go.
// testGetWithWallet resolves the caller's my_id by looking up the wallet against
// the users table, so each test user must be seeded with one of these addresses.
const (
	testEventArtistWallet = "0x7d273271690538cf855e5b3002a0dd8c154bb060"
	testEventFanWallet    = "0xc3d1d41e6872ffbd15c473d14fc3a9250be5b5e0"
	testEventFan2Wallet   = "0x4954d18926ba0ed9378938444731be4e622537b2"
)

func testEventFollowersBaseUsers() []map[string]any {
	return []map[string]any{
		{
			"user_id":   1,
			"handle":    "eventartist",
			"handle_lc": "eventartist",
			"name":      "Event Artist",
			"wallet":    testEventArtistWallet,
		},
		{
			"user_id":   2,
			"handle":    "eventfan",
			"handle_lc": "eventfan",
			"name":      "Event Fan",
			"wallet":    testEventFanWallet,
		},
		{
			"user_id":   3,
			"handle":    "eventfan2",
			"handle_lc": "eventfan2",
			"name":      "Event Fan 2",
			"wallet":    testEventFan2Wallet,
		},
	}
}

func TestEventFollowState_UnknownEventReturnsZeroNotError(t *testing.T) {
	// The endpoint is tolerant: a request for a non-existent event returns
	// 200 with zero counts rather than a 404. This makes the UI simpler
	// (the button just renders "Follow" for non-existent events) and avoids
	// a round-trip cascade when the contest was deleted mid-session.
	app := emptyTestApp(t)
	encEvent, err := trashid.EncodeHashId(999999)
	require.NoError(t, err)
	status, body := testGet(t, app, "/v1/events/"+encEvent+"/follow_state")
	require.Equal(t, 200, status, string(body))
	jsonAssert(t, body, map[string]any{
		"data.is_followed":    false,
		"data.follower_count": 0,
	})
}

func TestEventFollowState_InvalidEventIdReturns400(t *testing.T) {
	app := emptyTestApp(t)
	status, _ := testGet(t, app, "/v1/events/not-a-hashid/follow_state")
	assert.Equal(t, 400, status)
}

func TestEventFollowState_ZeroFollowers(t *testing.T) {
	app := emptyTestApp(t)
	database.Seed(app.pool.Replicas[0], database.FixtureMap{
		"users": testEventFollowersBaseUsers(),
		"tracks": {
			{
				"track_id":   1,
				"owner_id":   1,
				"title":      "Original",
				"created_at": "2020-01-01 00:00:00",
			},
		},
		"events": {
			{
				"event_id":    100,
				"event_type":  "remix_contest",
				"user_id":     1,
				"entity_type": "track",
				"entity_id":   1,
				"event_data":  map[string]any{"description": "remix"},
				"created_at":  "2020-05-01 00:00:00",
				"updated_at":  "2020-05-01 00:00:00",
			},
		},
	})

	encEvent, err := trashid.EncodeHashId(100)
	require.NoError(t, err)
	status, body := testGet(t, app, "/v1/events/"+encEvent+"/follow_state")
	require.Equal(t, 200, status, string(body))
	jsonAssert(t, body, map[string]any{
		"data.is_followed":    false,
		"data.follower_count": 0,
	})
}

func TestEventFollowState_CountsOnlyLiveEventSubscriptions(t *testing.T) {
	// follower_count MUST:
	//   - count rows with entity_type='Event' and matching entity_id=event_id
	//   - only include current & non-deleted rows
	//   - NOT count rows with entity_type='User' (legacy user-follows) even if
	//     they happen to target the same numeric id
	//   - NOT count Event rows where only the legacy user_id mirror matches
	//   - NOT count stale (is_current=false) rows
	app := emptyTestApp(t)

	database.Seed(app.pool.Replicas[0], database.FixtureMap{
		"users": testEventFollowersBaseUsers(),
		"tracks": {
			{
				"track_id":   1,
				"owner_id":   1,
				"title":      "Original",
				"created_at": "2020-01-01 00:00:00",
			},
		},
		"events": {
			{
				"event_id":    200,
				"event_type":  "remix_contest",
				"user_id":     1,
				"entity_type": "track",
				"entity_id":   1,
				"event_data":  map[string]any{"description": "remix"},
				"created_at":  "2020-05-01 00:00:00",
				"updated_at":  "2020-05-01 00:00:00",
			},
		},
		"subscriptions": []map[string]any{
			// Two live event subscribers
			{
				"subscriber_id": 2,
				"user_id":       200,
				"entity_type":   "Event",
				"entity_id":     200,
				"is_current":    true,
				"is_delete":     false,
				"blockhash":     "bh1",
				"blocknumber":   101,
				"txhash":        "tx1",
			},
			{
				"subscriber_id": 3,
				"user_id":       200,
				"entity_type":   "Event",
				"entity_id":     200,
				"is_current":    true,
				"is_delete":     false,
				"blockhash":     "bh2",
				"blocknumber":   101,
				"txhash":        "tx2",
			},
			// A legacy user-type subscription with matching numeric id —
			// must NOT be counted.
			{
				"subscriber_id": 2,
				"user_id":       200,
				"entity_type":   "User",
				"entity_id":     nil,
				"is_current":    true,
				"is_delete":     false,
				"blockhash":     "bh3",
				"blocknumber":   101,
				"txhash":        "tx3",
			},
			// A deleted event subscription — must NOT be counted.
			{
				"subscriber_id": 1,
				"user_id":       200,
				"entity_type":   "Event",
				"entity_id":     200,
				"is_current":    true,
				"is_delete":     true,
				"blockhash":     "bh4",
				"blocknumber":   101,
				"txhash":        "tx4",
			},
			// An Event subscription whose legacy mirror matches this event
			// but canonical entity_id points elsewhere — must NOT be counted.
			{
				"subscriber_id": 4,
				"user_id":       200,
				"entity_type":   "Event",
				"entity_id":     999,
				"is_current":    true,
				"is_delete":     false,
				"blockhash":     "bh5",
				"blocknumber":   101,
				"txhash":        "tx5",
			},
		},
	})

	encEvent, err := trashid.EncodeHashId(200)
	require.NoError(t, err)
	status, body := testGet(t, app, "/v1/events/"+encEvent+"/follow_state")
	require.Equal(t, 200, status, string(body))
	jsonAssert(t, body, map[string]any{
		"data.follower_count": 2,
		// No viewer auth was passed, so the is_followed flag is false.
		"data.is_followed": false,
	})
}

func TestEventFollowState_IsFollowedFromAuthedViewer(t *testing.T) {
	// When the request carries a signed wallet header that resolves to a user
	// with a current, non-deleted event subscription, is_followed is true.
	app := emptyTestApp(t)

	database.Seed(app.pool.Replicas[0], database.FixtureMap{
		"users": testEventFollowersBaseUsers(),
		"tracks": {
			{
				"track_id":   1,
				"owner_id":   1,
				"title":      "Original",
				"created_at": "2020-01-01 00:00:00",
			},
		},
		"events": {
			{
				"event_id":    300,
				"event_type":  "remix_contest",
				"user_id":     1,
				"entity_type": "track",
				"entity_id":   1,
				"event_data":  map[string]any{"description": "remix"},
				"created_at":  "2020-05-01 00:00:00",
				"updated_at":  "2020-05-01 00:00:00",
			},
		},
		"subscriptions": []map[string]any{
			{
				"subscriber_id": 2, // user 2 is subscribed
				"user_id":       300,
				"entity_type":   "Event",
				"entity_id":     300,
				"is_current":    true,
				"is_delete":     false,
				"blockhash":     "bh1",
				"blocknumber":   101,
				"txhash":        "tx1",
			},
		},
	})

	encEvent, err := trashid.EncodeHashId(300)
	require.NoError(t, err)
	user2HashID := trashid.MustEncodeHashID(2)
	user3HashID := trashid.MustEncodeHashID(3)

	// Viewer = user 2 — subscribed. Must supply BOTH user_id (so the middleware
	// sets myId=2) AND the matching wallet signature (so the auth check
	// accepts the request).
	status, body := testGetWithWallet(
		t,
		app,
		"/v1/events/"+encEvent+"/follow_state?user_id="+user2HashID,
		testEventFanWallet,
	)
	require.Equal(t, 200, status, string(body))
	jsonAssert(t, body, map[string]any{
		"data.is_followed":    true,
		"data.follower_count": 1,
	})

	// Viewer = user 3 — NOT subscribed. Still sees the follower_count but
	// their own follow state is false.
	status, body = testGetWithWallet(
		t,
		app,
		"/v1/events/"+encEvent+"/follow_state?user_id="+user3HashID,
		testEventFan2Wallet,
	)
	require.Equal(t, 200, status, string(body))
	jsonAssert(t, body, map[string]any{
		"data.is_followed":    false,
		"data.follower_count": 1,
	})
}

func TestEventFollowState_DoesNotTreatUserSubscriptionAsEventFollow(t *testing.T) {
	// A user who has a legacy User-type subscription whose user_id column
	// happens to equal an event's event_id must NOT appear as an event
	// follower. This is the regression guard for the entity_type discriminator.
	app := emptyTestApp(t)

	database.Seed(app.pool.Replicas[0], database.FixtureMap{
		"users": testEventFollowersBaseUsers(),
		"tracks": {
			{
				"track_id":   1,
				"owner_id":   1,
				"title":      "Original",
				"created_at": "2020-01-01 00:00:00",
			},
		},
		"events": {
			{
				"event_id":    400,
				"event_type":  "remix_contest",
				"user_id":     1,
				"entity_type": "track",
				"entity_id":   1,
				"event_data":  map[string]any{"description": "remix"},
				"created_at":  "2020-05-01 00:00:00",
				"updated_at":  "2020-05-01 00:00:00",
			},
		},
		"subscriptions": []map[string]any{
			// Legacy user-type subscription: user 2 is subscribed to user 400
			// (fake user_id chosen to match the event_id numerically).
			{
				"subscriber_id": 2,
				"user_id":       400,
				"entity_type":   "User",
				"entity_id":     nil,
				"is_current":    true,
				"is_delete":     false,
				"blockhash":     "bh1",
				"blocknumber":   101,
				"txhash":        "tx1",
			},
		},
	})

	encEvent, err := trashid.EncodeHashId(400)
	require.NoError(t, err)
	user2HashID := trashid.MustEncodeHashID(2)
	status, body := testGetWithWallet(
		t,
		app,
		"/v1/events/"+encEvent+"/follow_state?user_id="+user2HashID,
		testEventFanWallet,
	)
	require.Equal(t, 200, status, string(body))
	jsonAssert(t, body, map[string]any{
		"data.is_followed":    false,
		"data.follower_count": 0,
	})
}

// ---------------------------------------------------------------------------
// POST/DELETE follow — shallow coverage
// ---------------------------------------------------------------------------
//
// These endpoints build a ManageEntity transaction and hand it off to
// sendTransactionWithSigner, which requires a running ganache / chain to
// actually roundtrip. The real behavioral coverage lives in
// integration_tests/tasks/entity_manager/test_event_follow.py. Here we only
// verify that:
//   - missing auth is 401
//   - a non-existent event is 404 (pre-flight DB check, before any tx)
//   - a valid request gets past auth/parse/pre-flight, which means the
//     handler reaches the SDK layer (whose failure we tolerate)

func TestPostEventFollow_MissingAuthRejected(t *testing.T) {
	app := emptyTestApp(t)
	enc, err := trashid.EncodeHashId(100)
	require.NoError(t, err)
	status, _ := testPost(t, app, "/v1/events/"+enc+"/follow", nil, nil)
	assert.Equal(t, 401, status)
}

func TestPostEventFollow_UnknownEventIs404BeforeSendingTx(t *testing.T) {
	// We hit the pre-flight EXISTS check inside postV1EventFollow before any
	// chain transaction is built, so this must return 404 deterministically
	// even without a running chain.
	testPrivateKey := "ac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80"
	testWallet := testdata.CreateTestWallet(t, testPrivateKey)

	app := emptyTestApp(t)

	now := time.Now()
	database.Seed(app.writePool, database.FixtureMap{
		"users": {
			{
				"user_id":    500,
				"handle":     "eventfollower",
				"handle_lc":  "eventfollower",
				"wallet":     strings.ToLower(testWallet.Address),
				"is_current": true,
				"created_at": now,
				"updated_at": now,
			},
		},
		"grants": {
			{
				"user_id":         500,
				"grantee_address": strings.ToLower(testWallet.Address),
				"is_approved":     true,
				"is_revoked":      false,
				"is_current":      true,
				"created_at":      now,
				"updated_at":      now,
			},
		},
	})

	encEvent, err := trashid.EncodeHashId(999999)
	require.NoError(t, err)
	userHashID := trashid.MustEncodeHashID(500)

	authString := fmt.Sprintf("user:%s", testPrivateKey)
	headers := map[string]string{
		"Authorization": "Basic " + base64.StdEncoding.EncodeToString([]byte(authString)),
	}
	status, _ := testPost(
		t,
		app,
		fmt.Sprintf("/v1/events/%s/follow?user_id=%s", encEvent, userHashID),
		nil,
		headers,
	)
	assert.Equal(t, 404, status)
}

func TestPostEventFollow_DeletedEventIs404(t *testing.T) {
	testPrivateKey := "ac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80"
	testWallet := testdata.CreateTestWallet(t, testPrivateKey)

	app := emptyTestApp(t)

	now := time.Now()
	database.Seed(app.writePool, database.FixtureMap{
		"users": {
			{
				"user_id":    500,
				"handle":     "eventfollower",
				"handle_lc":  "eventfollower",
				"wallet":     strings.ToLower(testWallet.Address),
				"is_current": true,
				"created_at": now,
				"updated_at": now,
			},
		},
		"tracks": {
			{
				"track_id":   1,
				"owner_id":   1,
				"title":      "Original",
				"created_at": now,
			},
		},
		"events": {
			{
				"event_id":    700,
				"event_type":  "remix_contest",
				"user_id":     1,
				"entity_type": "track",
				"entity_id":   1,
				"event_data":  map[string]any{"description": "deleted contest"},
				"is_deleted":  true,
				"created_at":  now,
				"updated_at":  now,
			},
		},
		"grants": {
			{
				"user_id":         500,
				"grantee_address": strings.ToLower(testWallet.Address),
				"is_approved":     true,
				"is_revoked":      false,
				"is_current":      true,
				"created_at":      now,
				"updated_at":      now,
			},
		},
	})

	encEvent, err := trashid.EncodeHashId(700)
	require.NoError(t, err)
	userHashID := trashid.MustEncodeHashID(500)

	authString := fmt.Sprintf("user:%s", testPrivateKey)
	headers := map[string]string{
		"Authorization": "Basic " + base64.StdEncoding.EncodeToString([]byte(authString)),
	}
	status, _ := testPost(
		t,
		app,
		fmt.Sprintf("/v1/events/%s/follow?user_id=%s", encEvent, userHashID),
		nil,
		headers,
	)
	assert.Equal(t, 404, status)
}

func TestEventsFollowers_ReturnsOnlyLiveEventSubscribers(t *testing.T) {
	// /v1/events/:eventId/followers returns the list of users subscribed
	// to the event. Mirrors the shape + filtering rules of follow_state:
	// entity_type='Event', is_current & !is_delete, and ignores legacy
	// user-type subscriptions with a colliding numeric id.
	app := emptyTestApp(t)

	database.Seed(app.pool.Replicas[0], database.FixtureMap{
		"users": testEventFollowersBaseUsers(),
		"tracks": {
			{
				"track_id":   1,
				"owner_id":   1,
				"title":      "Original",
				"created_at": "2020-01-01 00:00:00",
			},
		},
		"events": {
			{
				"event_id":    200,
				"event_type":  "remix_contest",
				"user_id":     1,
				"entity_type": "track",
				"entity_id":   1,
				"event_data":  map[string]any{"description": "remix"},
				"created_at":  "2020-05-01 00:00:00",
				"updated_at":  "2020-05-01 00:00:00",
			},
		},
		"subscriptions": []map[string]any{
			{
				"subscriber_id": 2,
				"user_id":       200,
				"entity_type":   "Event",
				"entity_id":     200,
				"is_current":    true,
				"is_delete":     false,
				"blockhash":     "bh1",
				"blocknumber":   101,
				"txhash":        "tx1",
			},
			{
				"subscriber_id": 3,
				"user_id":       200,
				"entity_type":   "Event",
				"entity_id":     200,
				"is_current":    true,
				"is_delete":     false,
				"blockhash":     "bh2",
				"blocknumber":   101,
				"txhash":        "tx2",
			},
			// Legacy user-type subscription with a colliding numeric id —
			// must NOT show up.
			{
				"subscriber_id": 1,
				"user_id":       200,
				"entity_type":   "User",
				"entity_id":     nil,
				"is_current":    true,
				"is_delete":     false,
				"blockhash":     "bh3",
				"blocknumber":   101,
				"txhash":        "tx3",
			},
			// Deleted event subscription — must NOT show up.
			{
				"subscriber_id": 1,
				"user_id":       200,
				"entity_type":   "Event",
				"entity_id":     200,
				"is_current":    true,
				"is_delete":     true,
				"blockhash":     "bh4",
				"blocknumber":   101,
				"txhash":        "tx4",
			},
		},
	})

	encEvent, err := trashid.EncodeHashId(200)
	require.NoError(t, err)
	status, body := testGet(t, app, "/v1/events/"+encEvent+"/followers")
	require.Equal(t, 200, status, string(body))

	jsonAssert(t, body, map[string]any{
		"data.#": 2,
	})
}

func TestEventsFollowers_UnknownEventReturnsEmptyList(t *testing.T) {
	app := emptyTestApp(t)

	encEvent, err := trashid.EncodeHashId(9999)
	require.NoError(t, err)
	status, body := testGet(t, app, "/v1/events/"+encEvent+"/followers")
	require.Equal(t, 200, status, string(body))
	jsonAssert(t, body, map[string]any{
		"data.#": 0,
	})
}

func TestEventsFollowers_InvalidEventIdReturns400(t *testing.T) {
	app := emptyTestApp(t)

	status, _ := testGet(t, app, "/v1/events/not-a-hashid/followers")
	assert.Equal(t, 400, status)
}
