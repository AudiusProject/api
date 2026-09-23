package api

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	comms "api.audius.co/api/comms"
	"api.audius.co/api/testdata"
	"api.audius.co/database"
	"api.audius.co/trashid"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

// dummy pkeys generated from ganache "test test...junk" seed
var user1WalletKey = "0xac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80"
var user2WalletKey = "0x59c6995e998f97a5a0044966f0945389dc9e86dae88c7a8412f4603b6b78690d"

// createSignedRPCPayload creates a signed RPC payload for testing
func postMutateRPCData(t *testing.T, app *ApiServer, currentUserID string, method comms.RPCMethod, params any, timestamp int64, wallet *testdata.TestWallet) (int, []byte) {
	paramsBytes, err := json.Marshal(params)
	if err != nil {
		t.Fatalf("Failed to marshal params: %v", err)
	}

	rpcBytes, err := json.Marshal(comms.RawRPC{
		CurrentUserID: currentUserID,
		Method:        string(method),
		Params:        paramsBytes,
		Timestamp:     timestamp,
	})
	if err != nil {
		t.Fatalf("Failed to marshal rpcData: %v", err)
	}

	signature, err := wallet.SignData(rpcBytes)
	if err != nil {
		t.Fatalf("Failed to sign data: %v", err)
	}

	status, body := testPost(t, app, "/comms/mutate", rpcBytes, map[string]string{
		comms.SigHeader: signature,
	})

	return status, body
}

// This file is testing basic functionality of posting RPC messages to the mutation
// endpoint. There are more comprehensive tests of the internal logic (migrated from
// protocol repo) in the comms package
func TestPostMutateChat(t *testing.T) {
	testWallet1 := testdata.CreateTestWallet(t, user1WalletKey)
	app := emptyTestApp(t)

	// Setup test data
	now := time.Now()
	fixtures := database.FixtureMap{
		"users": {
			{
				"user_id":    1,
				"handle":     "user1",
				"wallet":     strings.ToLower(testWallet1.Address),
				"created_at": now.Add(-time.Hour),
				"updated_at": now.Add(-time.Hour),
				"is_current": true,
			},
			{
				"user_id":    2,
				"handle":     "user2",
				"wallet":     "0x7d273271690538cf855e5b3002a0dd8c154bb060",
				"created_at": now.Add(-time.Hour),
				"updated_at": now.Add(-time.Hour),
				"is_current": true,
			},
		},
	}

	database.Seed(app.writePool, fixtures)

	var user1EncodedID = trashid.MustEncodeHashID(1)
	var user2EncodedID = trashid.MustEncodeHashID(2)

	t.Run("valid create, skip dupes", func(t *testing.T) {
		chatId := trashid.ChatID(1, 2)
		params := comms.ChatCreateRPCParams{
			ChatID: chatId,
			Invites: []comms.PurpleInvite{
				{
					UserID:     user1EncodedID,
					InviteCode: "test",
				},
				{
					UserID:     user2EncodedID,
					InviteCode: "test",
				},
			},
		}

		{
			status, _ := postMutateRPCData(t, app, user1EncodedID, comms.RPCMethodChatCreate, params, now.UnixMilli(), testWallet1)
			assert.Equal(t, 200, status)

			url := fmt.Sprintf("/comms/chats/%s", chatId)

			status, body := testGetWithWallet(t, app, url, "0x7d273271690538cf855e5b3002a0dd8c154bb060")
			assert.Equal(t, 200, status)
			jsonAssert(t, body, map[string]any{
				"data.invite_code":            "test",
				"data.chat_members.0.user_id": user1EncodedID,
				"data.chat_members.1.user_id": user2EncodedID,
			})
		}

		{
			// Create same chat again, should fail
			status, _ := postMutateRPCData(t, app, user1EncodedID, comms.RPCMethodChatCreate, params, now.UnixMilli(), testWallet1)
			assert.Equal(t, 400, status)
		}
	})
}

// testGetWithTestWallet makes a GET request authenticated with signature
// headers signed by the given TestWallet (EIP-191 personal message), for
// wallets that do not have canned signatures in testdata.TestSignatures.
func testGetWithTestWallet(t *testing.T, app *ApiServer, path string, wallet *testdata.TestWallet) (int, []byte) {
	t.Helper()

	message := fmt.Sprintf("signature:%d", time.Now().UnixMilli())
	prefixedMsg := []byte(fmt.Sprintf("\x19Ethereum Signed Message:\n%d%s", len(message), message))
	finalHash := crypto.Keccak256Hash(prefixedMsg)
	sigBytes, err := crypto.Sign(finalHash.Bytes(), wallet.PrivateKey)
	require.NoError(t, err)

	req := httptest.NewRequest("GET", path, nil)
	req.Header.Set("Encoded-Data-Message", message)
	req.Header.Set("Encoded-Data-Signature", "0x"+hex.EncodeToString(sigBytes))

	res, err := app.Test(req, -1)
	require.NoError(t, err)
	body, _ := io.ReadAll(res.Body)
	return res.StatusCode, body
}

func TestPostMutateChatSetCategory(t *testing.T) {
	testWallet1 := testdata.CreateTestWallet(t, user1WalletKey)
	app := emptyTestApp(t)

	now := time.Now()
	database.Seed(app.writePool, database.FixtureMap{
		"users": {
			{
				"user_id":    1,
				"handle":     "user1",
				"wallet":     strings.ToLower(testWallet1.Address),
				"created_at": now.Add(-time.Hour),
				"updated_at": now.Add(-time.Hour),
				"is_current": true,
			},
			{
				"user_id":    2,
				"handle":     "user2",
				"wallet":     "0x7d273271690538cf855e5b3002a0dd8c154bb060",
				"created_at": now.Add(-time.Hour),
				"updated_at": now.Add(-time.Hour),
				"is_current": true,
			},
		},
	})

	user1EncodedID := trashid.MustEncodeHashID(1)
	user2EncodedID := trashid.MustEncodeHashID(2)
	chatId := trashid.ChatID(1, 2)
	chatUrl := fmt.Sprintf("/comms/chats/%s", chatId)

	// create the chat and send a message so it shows up in GET /comms/chats
	// (which requires last_message to be set)
	{
		status, _ := postMutateRPCData(t, app, user1EncodedID, comms.RPCMethodChatCreate, comms.ChatCreateRPCParams{
			ChatID: chatId,
			Invites: []comms.PurpleInvite{
				{UserID: user1EncodedID, InviteCode: "test"},
				{UserID: user2EncodedID, InviteCode: "test"},
			},
		}, now.UnixMilli(), testWallet1)
		require.Equal(t, 200, status)

		status, _ = postMutateRPCData(t, app, user1EncodedID, comms.RPCMethodChatMessage, comms.ChatMessageRPCParams{
			ChatID:    chatId,
			MessageID: "msg1",
			Message:   "hello",
		}, now.Add(time.Second).UnixMilli(), testWallet1)
		require.Equal(t, 200, status)
	}

	// uncategorized by default
	{
		status, body := testGetWithTestWallet(t, app, chatUrl, testWallet1)
		require.Equal(t, 200, status)
		jsonAssert(t, body, map[string]any{
			"data.chat_id":  chatId,
			"data.category": nil,
		})
		// the key must be present (null), not omitted
		assert.True(t, gjson.GetBytes(body, "data.category").Exists())

		status, body = testGetWithTestWallet(t, app, "/comms/chats", testWallet1)
		require.Equal(t, 200, status)
		jsonAssert(t, body, map[string]any{
			"data.0.chat_id":  chatId,
			"data.0.category": nil,
		})
	}

	// set category to "priority"
	{
		category := "priority"
		status, _ := postMutateRPCData(t, app, user1EncodedID, comms.RPCMethodChatSetCategory, comms.ChatSetCategoryRPCParams{
			ChatID:   chatId,
			Category: &category,
		}, now.Add(2*time.Second).UnixMilli(), testWallet1)
		require.Equal(t, 200, status)

		status, body := testGetWithTestWallet(t, app, chatUrl, testWallet1)
		require.Equal(t, 200, status)
		jsonAssert(t, body, map[string]any{"data.category": "priority"})

		status, body = testGetWithTestWallet(t, app, "/comms/chats", testWallet1)
		require.Equal(t, 200, status)
		jsonAssert(t, body, map[string]any{"data.0.category": "priority"})

		// the category is per-user: user 2 still sees the chat as uncategorized
		status, body = testGetWithWallet(t, app, chatUrl, "0x7d273271690538cf855e5b3002a0dd8c154bb060")
		require.Equal(t, 200, status)
		jsonAssert(t, body, map[string]any{"data.category": nil})
	}

	// change to "general"
	{
		category := "general"
		status, _ := postMutateRPCData(t, app, user1EncodedID, comms.RPCMethodChatSetCategory, comms.ChatSetCategoryRPCParams{
			ChatID:   chatId,
			Category: &category,
		}, now.Add(3*time.Second).UnixMilli(), testWallet1)
		require.Equal(t, 200, status)

		status, body := testGetWithTestWallet(t, app, chatUrl, testWallet1)
		require.Equal(t, 200, status)
		jsonAssert(t, body, map[string]any{"data.category": "general"})
	}

	// invalid category is rejected by the validator
	{
		category := "spam"
		status, _ := postMutateRPCData(t, app, user1EncodedID, comms.RPCMethodChatSetCategory, comms.ChatSetCategoryRPCParams{
			ChatID:   chatId,
			Category: &category,
		}, now.Add(4*time.Second).UnixMilli(), testWallet1)
		assert.Equal(t, 400, status)

		status, body := testGetWithTestWallet(t, app, chatUrl, testWallet1)
		require.Equal(t, 200, status)
		jsonAssert(t, body, map[string]any{"data.category": "general"})
	}

	// null clears the category
	{
		status, _ := postMutateRPCData(t, app, user1EncodedID, comms.RPCMethodChatSetCategory, comms.ChatSetCategoryRPCParams{
			ChatID:   chatId,
			Category: nil,
		}, now.Add(5*time.Second).UnixMilli(), testWallet1)
		require.Equal(t, 200, status)

		status, body := testGetWithTestWallet(t, app, chatUrl, testWallet1)
		require.Equal(t, 200, status)
		jsonAssert(t, body, map[string]any{"data.category": nil})
		assert.True(t, gjson.GetBytes(body, "data.category").Exists())
	}
}

func TestGetUnreadCountByCategory(t *testing.T) {
	app := emptyTestApp(t)

	now := time.Now()
	wallet := "0x7d273271690538cf855e5b3002a0dd8c154bb060"
	url := "/comms/chats/unread_by_category"

	// user 1 is the current user; user 2 is the other member of every chat
	fixtures := database.FixtureMap{
		"users": {
			{"user_id": 1, "handle": "user1", "wallet": wallet, "created_at": now, "updated_at": now, "is_current": true},
			{"user_id": 2, "handle": "user2", "wallet": "wallet2", "created_at": now, "updated_at": now, "is_current": true},
		},
		"chat":                          {},
		"chat_member":                   {},
		"user_conversation_preferences": {},
	}

	// (chat_id, category for user 1, unread_count for user 1)
	type chatSpec struct {
		id       string
		category string // "" = uncategorized
		unread   int
	}
	specs := []chatSpec{
		{"chat_priority_1", "priority", 3},
		{"chat_priority_2", "priority", 1},
		{"chat_priority_read", "priority", 0}, // read: not counted
		{"chat_general_1", "general", 5},
		{"chat_general_read", "general", 0}, // read: not counted
		{"chat_uncat_1", "", 2},
		{"chat_uncat_2", "", 1},
		{"chat_uncat_3", "", 1},
		{"chat_uncat_read", "", 0}, // read: not counted
	}
	for _, s := range specs {
		fixtures["chat"] = append(fixtures["chat"], map[string]any{
			"chat_id": s.id, "last_message": "hi", "last_message_at": now,
		})
		fixtures["chat_member"] = append(fixtures["chat_member"],
			map[string]any{"chat_id": s.id, "user_id": 1, "invited_by_user_id": 1, "invite_code": "x", "unread_count": s.unread},
			// the other member has everything unread, but that must not count for user 1
			map[string]any{"chat_id": s.id, "user_id": 2, "invited_by_user_id": 1, "invite_code": "x", "unread_count": 9},
		)
		if s.category != "" {
			fixtures["user_conversation_preferences"] = append(fixtures["user_conversation_preferences"], map[string]any{
				"user_id": 1, "chat_id": s.id, "category": s.category,
			})
		}
	}
	// user 2's own preference for a chat must not affect user 1's counts
	fixtures["user_conversation_preferences"] = append(fixtures["user_conversation_preferences"], map[string]any{
		"user_id": 2, "chat_id": "chat_uncat_1", "category": "priority",
	})

	database.Seed(app.writePool, fixtures)

	status, body := testGetWithWallet(t, app, url, wallet)
	require.Equal(t, 200, status)
	jsonAssert(t, body, map[string]any{
		"data.priority":      2,
		"data.general":       1,
		"data.uncategorized": 3,
	})

	// sanity: the plain unread endpoint agrees with the sum
	status, body = testGetWithWallet(t, app, "/comms/chats/unread", wallet)
	require.Equal(t, 200, status)
	jsonAssert(t, body, map[string]any{"data": 6})

	// a user with no unread chats gets all three keys, each zero
	database.Seed(app.writePool, database.FixtureMap{
		"users": {
			{"user_id": 3, "handle": "user3", "wallet": "0xc3d1d41e6872ffbd15c473d14fc3a9250be5b5e0", "created_at": now, "updated_at": now, "is_current": true},
		},
	})
	status, body = testGetWithWallet(t, app, url, "0xc3d1d41e6872ffbd15c473d14fc3a9250be5b5e0")
	require.Equal(t, 200, status)
	jsonAssert(t, body, map[string]any{
		"data.priority":      0,
		"data.general":       0,
		"data.uncategorized": 0,
	})
	for _, key := range []string{"data.priority", "data.general", "data.uncategorized"} {
		assert.True(t, gjson.GetBytes(body, key).Exists(), "%s must always be present", key)
	}
}
