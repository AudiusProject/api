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

func TestPostMutateChat(t *testing.T) {
	testWallet1, err := testdata.CreateTestWallet(user1WalletKey)
	if err != nil {
		t.Fatalf("Failed to create test wallet: %v", err)
	}
	testWallet2, err := testdata.CreateTestWallet(user2WalletKey)
	if err != nil {
		t.Fatalf("Failed to create test wallet: %v", err)
	}
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

	t.Run("chat create", func(t *testing.T) {
		params, err := json.Marshal(comms.ChatCreateRPCParams{
			ChatID: user1EncodedID + ":" + user2EncodedID,
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
		})
		if err != nil {
			t.Fatalf("Failed to marshal params: %v", err)
		}

		rpcBytes, err := json.Marshal(comms.RawRPC{
			CurrentUserID: user1EncodedID,
			Method:        string(comms.RPCMethodChatCreate),
			Params:        params,
			Timestamp:     now.UnixMilli(),
		})

		if err != nil {
			t.Fatalf("Failed to marshal rpcData: %v", err)
		}

		signature, err := testWallet1.SignData(rpcBytes)

		// Test getting regular chat messages (not blasts)
		status, _ := testPost(t, app, "/comms/mutate", rpcBytes, map[string]string{
			comms.SigHeader: signature,
		})
		assert.Equal(t, 200, status)

		// TODO: Check body for rpc log?
	})
}
