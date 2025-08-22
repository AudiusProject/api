package api

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	comms "bridgerton.audius.co/api/comms"
	"bridgerton.audius.co/api/testdata"
	"bridgerton.audius.co/database"
	"bridgerton.audius.co/trashid"
	"github.com/stretchr/testify/assert"
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
	testWallet2 := testdata.CreateTestWallet(t, user2WalletKey)
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
				"wallet":     strings.ToLower(testWallet2.Address),
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
		params := comms.ChatCreateRPCParams{
			ChatID: trashid.ChatID(1, 2),
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
			// Test getting regular chat messages (not blasts)
			status, _ := postMutateRPCData(t, app, user1EncodedID, comms.RPCMethodChatCreate, params, now.UnixMilli(), testWallet1)
			assert.Equal(t, 200, status)
			// TODO: Fetch and check it
		}

		{
			// Create same chat again, should fail
			status, _ := postMutateRPCData(t, app, user1EncodedID, comms.RPCMethodChatCreate, params, now.UnixMilli(), testWallet1)
			assert.Equal(t, 400, status)
		}
	})
}

/* TODO:
- 403 when attestation fails
- 400 when we can't get user id from wallet
- 400 when readSignedPost fails
*/
