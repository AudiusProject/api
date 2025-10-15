package token

import (
	"encoding/json"
	"os"
	"testing"

	"api.audius.co/database"
	"api.audius.co/solana/indexer/common"
	"api.audius.co/solana/indexer/fake_rpc_client"
	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
	"github.com/jackc/pgx/v5"
	"github.com/maypok86/otter"
	pb "github.com/rpcpool/yellowstone-grpc/examples/golang/proto"
	"github.com/test-go/testify/assert"
	"github.com/test-go/testify/require"
	"go.uber.org/zap"
)

func TestHandleUpdate_SlotCheckpoint(t *testing.T) {
	pool := database.CreateTestDatabase(t, "test_solana_indexer_damm_v2")
	rpcClient := fake_rpc_client.FakeRpcClient{}
	logger := zap.NewNop()

	indexer := New(common.GrpcConfig{}, &rpcClient, pool, nil, logger)

	expectedSlot := uint64(1500)

	request := pb.SubscribeRequest{}
	checkpointId, err := common.InsertCheckpointStart(t.Context(), pool, "test", 1000, &request)
	update := pb.SubscribeUpdate{
		Filters: []string{checkpointId},
		UpdateOneof: &pb.SubscribeUpdate_Slot{
			Slot: &pb.SubscribeUpdateSlot{
				Slot: expectedSlot,
			},
		},
	}

	indexer.HandleUpdate(t.Context(), &update)

	slot, err := common.GetCheckpointSlot(t.Context(), pool, "test", &request)
	require.NoError(t, err)
	assert.Equal(t, expectedSlot, slot, "checkpoint slot should be updated")
}

func TestHandleUpdate_BalanceChange(t *testing.T) {
	pool := database.CreateTestDatabase(t, "test_solana_indexer_token")

	mint := "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v"
	sender := "F1vVY6VtF5oLT2QYEqy6276JGGhgaLEDZMamoFsJWSYk"
	senderTokenAccount := "DUiUiDme6XoqaD86AdmqY2BDSg3PrCidszKpNbZhfkpo"
	expectedSenderBalance := uint64(3300)

	sender2 := "2bX4g7yV3aHjv6v1d8Z8n5K5e5f5L5e5f5L5e5f5L5e5"
	sender2TokenAccount := "8bX4g7yV3aHjv6v1d8Z8n5K5e5f5L5e5f5L5e5f5L5e5"
	expectedSender2Balance := uint64(1000)

	receiver := "FFwKgUzzmvFv1mhqexs2muRAphgMMyR1kMtiigPeoksw"
	receiverTokenAccount := "AaF7Y7PCk54xrBvbwJEbGY8p5FnZ2zjzzPRnY4VsF17n"
	expectedReceiverBalance := uint64(1430000)

	database.Seed(pool, database.FixtureMap{
		"users": []map[string]any{
			{"user_id": 1, "wallet": "0x123"},
			{"user_id": 2},
		},
		"associated_wallets": []map[string]any{
			{"id": 1, "user_id": 1, "wallet": sender2, "chain": "sol"},
			{"id": 2, "user_id": 2, "wallet": receiver, "chain": "sol"},
		},
		"sol_claimable_accounts": []map[string]any{
			{"signature": "abc", "ethereum_address": "0x123", "account": senderTokenAccount, "mint": mint},
		},
		"sol_token_account_balances": []map[string]any{
			{"account": sender2TokenAccount, "mint": mint, "owner": sender2, "balance": expectedSender2Balance},
		},
	})

	rpcClient := fake_rpc_client.FakeRpcClient{}
	transactionCache, err := otter.MustBuilder[solana.Signature, *rpc.GetTransactionResult](10).Build()
	require.NoError(t, err, "failed to create cache")
	logger := zap.NewNop()

	txSig := solana.MustSignatureFromBase58("3HKyrGEH5nDJfMfuJk5cNEBSxNWZb3yBNksqFgEy1XLhwMV31oxyiG5Ju84i6EEYp2gk8EhDohroPrDGVWsK6hJY")
	slot := uint64(373471965)

	txJson, err := os.ReadFile("./claimable_tokens_transaction_test_fixture.json")
	require.NoError(t, err, "failed to read transaction test fixture")

	var txResult rpc.GetTransactionResult
	err = json.Unmarshal(txJson, &txResult)
	require.NoError(t, err, "failed to unmarshal transaction test fixture")

	transactionCache.Set(txSig, &txResult)

	update := pb.SubscribeUpdate{
		Filters: []string{mint},
		UpdateOneof: &pb.SubscribeUpdate_Account{
			Account: &pb.SubscribeUpdateAccount{
				Slot: slot,
				Account: &pb.SubscribeUpdateAccountInfo{
					TxnSignature: txSig[:],
				},
			},
		},
	}

	indexer := New(common.GrpcConfig{}, &rpcClient, pool, &transactionCache, logger)
	err = indexer.HandleUpdate(t.Context(), &update)
	require.NoError(t, err, "failed to handle update")

	// Check that balance changes exist
	sql := `
		SELECT EXISTS (
			SELECT 1
			FROM sol_token_account_balance_changes
			WHERE signature = @signature
				AND mint = @mint
				AND owner = @owner
				AND account = @account
				AND change = @change
				AND balance = @balance
				AND slot = @slot
			LIMIT 1
		)
	`

	// Sender balance change
	var exists bool
	err = pool.QueryRow(t.Context(), sql, pgx.NamedArgs{
		"signature": txSig.String(),
		"mint":      mint,
		"owner":     sender,
		"account":   senderTokenAccount,
		"change":    int64(-90000),
		"balance":   expectedSenderBalance,
		"slot":      slot,
	}).Scan(&exists)
	require.NoError(t, err, "failed to query for balance change")
	assert.True(t, exists, "balance change should exist")

	// Receiver balance change
	err = pool.QueryRow(t.Context(), sql, pgx.NamedArgs{
		"signature": txSig.String(),
		"mint":      mint,
		"owner":     receiver,
		"account":   receiverTokenAccount,
		"change":    int64(90000),
		"balance":   expectedReceiverBalance,
		"slot":      slot,
	}).Scan(&exists)
	require.NoError(t, err, "failed to query for balance change")
	assert.True(t, exists, "balance change should exist")

	// Check that token account balances are updated
	sql = `
		SELECT EXISTS (
			SELECT 1
			FROM sol_token_account_balances
			WHERE account = @account
				AND mint = @mint
				AND owner = @owner
				AND balance = @balance
		)
	`

	// Sender balance
	err = pool.QueryRow(t.Context(), sql, pgx.NamedArgs{
		"account": senderTokenAccount,
		"mint":    mint,
		"owner":   sender,
		"balance": expectedSenderBalance,
	}).Scan(&exists)
	require.NoError(t, err, "failed to query for sender balance")
	assert.True(t, exists, "sender balance should be updated")

	// Receiver balance
	err = pool.QueryRow(t.Context(), sql, pgx.NamedArgs{
		"account": receiverTokenAccount,
		"mint":    mint,
		"owner":   receiver,
		"balance": expectedReceiverBalance,
	}).Scan(&exists)
	require.NoError(t, err, "failed to query for receiver balance")
	assert.True(t, exists, "receiver balance should be updated")

	// Check user balances are updated
	sql = `
		SELECT EXISTS (
			SELECT 1
			FROM sol_user_balances
			WHERE user_id = @userId
				AND mint = @mint
				AND balance = @balance
			LIMIT 1
		)
	`

	// Sender user balance
	err = pool.QueryRow(t.Context(), sql, pgx.NamedArgs{
		"userId":  1,
		"mint":    mint,
		"balance": expectedSenderBalance + expectedSender2Balance,
	}).Scan(&exists)
	require.NoError(t, err, "failed to query for sender user balance")
	assert.True(t, exists, "sender user balance should be updated")

	// Receiver user balance
	err = pool.QueryRow(t.Context(), sql, pgx.NamedArgs{
		"userId":  2,
		"mint":    mint,
		"balance": expectedReceiverBalance,
	}).Scan(&exists)
	require.NoError(t, err, "failed to query for receiver user balance")
	assert.True(t, exists, "receiver user balance should be updated")
}
