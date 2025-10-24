package locker

import (
	"encoding/base64"
	"testing"

	"api.audius.co/database"
	"api.audius.co/solana/indexer/common"
	"api.audius.co/solana/indexer/fake_rpc_client"
	"github.com/gagliardetto/solana-go"
	"github.com/jackc/pgx/v5"
	pb "github.com/rpcpool/yellowstone-grpc/examples/golang/proto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestHandleUpdate_SlotCheckpoint(t *testing.T) {
	pool := database.CreateTestDatabase(t, "test_solana_indexer_dbc")
	rpcClient := fake_rpc_client.FakeRpcClient{}
	logger := zap.NewNop()

	indexer := New(common.GrpcConfig{}, &rpcClient, pool, logger)

	expectedSlot := uint64(1500)

	request := pb.SubscribeRequest{}
	checkpointId, err := common.InsertCheckpointStart(t.Context(), pool, "test", 1000, &request)
	require.NoError(t, err)

	update := pb.SubscribeUpdate{
		Filters: []string{checkpointId},
		UpdateOneof: &pb.SubscribeUpdate_Slot{
			Slot: &pb.SubscribeUpdateSlot{
				Slot: expectedSlot,
			},
		},
	}

	err = indexer.HandleUpdate(t.Context(), &update)
	require.NoError(t, err)

	slot, err := common.GetCheckpointSlot(t.Context(), pool, "test", &request)
	require.NoError(t, err)
	assert.Equal(t, expectedSlot, slot, "checkpoint slot should be updated")
}

func TestHandleUpdate_VestingEscrow(t *testing.T) {
	pool := database.CreateTestDatabase(t, "test_solana_indexer_dbc")
	rpcClient := fake_rpc_client.FakeRpcClient{}
	logger := zap.NewNop()

	indexer := New(common.GrpcConfig{}, &rpcClient, pool, logger)

	// Real VestingEscrow account data taken from prod
	escrowAddress := solana.MustPublicKeyFromBase58("7fXoYtLh1bG7q3Yh3b3Y9T5oX5nU5Y1Z6L8v1K5vU6Lm")
	escrowBase64 := "9He3BEl0h8Pma7+K8q1+Y5zeZOJijiDxit+NwzCo6fSDuaAK1rtuUHcaLxqg1AjgXzqUrEhTyula0B3153h+c/pj9a/R+ETB2mNoH3KGvMgGcZ4sK1CiAVckO/yWaLEVILxThD7c2wiAwc02YNTKuGhqgHA0AyospRPz5Pq9NNsMGdSqUKuLgf4CAQAAAAAAwZvyaAAAAACAUQEAAAAAAAAAAAAAAAAADhyqNy35AAAhBwAAAAAAAAYAAAAAAAAAwZvyaAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA="
	escrowData, err := base64.StdEncoding.DecodeString(escrowBase64)
	require.NoError(t, err)

	update := pb.SubscribeUpdate{
		UpdateOneof: &pb.SubscribeUpdate_Account{
			Account: &pb.SubscribeUpdateAccount{
				Slot: 600000000,
				Account: &pb.SubscribeUpdateAccountInfo{
					Pubkey:       escrowAddress.Bytes(),
					Data:         escrowData,
					TxnSignature: nil,
				},
			},
		},
	}

	err = indexer.HandleUpdate(t.Context(), &update)
	require.NoError(t, err)

	// Verify that the vesting escrow was inserted
	sql := `
		SELECT EXISTS (
			SELECT 1
			FROM sol_locker_vesting_escrows
			WHERE account = @account
				AND slot = @slot
				AND recipient = @recipient
				AND token_mint = @token_mint
				AND creator = @creator
				AND base = @base
				AND escrow_bump = @escrow_bump
				AND update_recipient_mode = @update_recipient_mode
				AND cancel_mode = @cancel_mode
				AND token_program_flag = @token_program_flag
				AND cliff_time = @cliff_time
				AND frequency = @frequency
				AND cliff_unlock_amount = @cliff_unlock_amount
				AND amount_per_period = @amount_per_period
				AND number_of_period = @number_of_period
				AND total_claimed_amount = @total_claimed_amount
				AND vesting_start_time = @vesting_start_time
				AND cancelled_at = @cancelled_at
			LIMIT 1
		)
	`
	var exists bool
	err = pool.QueryRow(t.Context(), sql, pgx.NamedArgs{
		"account":               escrowAddress.String(),
		"slot":                  int64(600000000),
		"recipient":             "GWU4gnhaGPdhh4mcXudMpBXRUVJxqRPPXaSc8UkU2D3u",
		"token_mint":            "91vg3y8HsmcShJARjpEMZBu5z2W5BwY4fdCj46QZCCnk",
		"creator":               "FhVo3mqL8PW5pH5U2CN4XE33DokiyZnUwuGpH2hmHLuM",
		"base":                  "9fcau4PNu4JGuS5J8dqP2qJiQbzxu5KeV12a94khdTma",
		"escrow_bump":           uint8(254),
		"update_recipient_mode": int8(2),
		"cancel_mode":           int8(1),
		"token_program_flag":    int8(0),
		"cliff_time":            int64(1760730049),
		"frequency":             int64(86400),
		"cliff_unlock_amount":   uint64(0),
		"amount_per_period":     uint64(273972602739726),
		"number_of_period":      uint64(1825),
		"total_claimed_amount":  uint64(6),
		"vesting_start_time":    int64(1760730049),
		"cancelled_at":          int64(0),
	}).Scan(&exists)
	require.NoError(t, err, "failed to query for vesting escrow")
	assert.True(t, exists, "vesting escrow should exist after indexing")
}
