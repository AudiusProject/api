package program

import (
	"encoding/json"
	"os"
	"testing"

	"api.audius.co/config"
	"api.audius.co/database"
	"api.audius.co/solana/indexer/common"
	"api.audius.co/solana/indexer/fake_rpc_client"
	"api.audius.co/solana/spl/programs/claimable_tokens"
	"api.audius.co/solana/spl/programs/payment_router"
	"api.audius.co/solana/spl/programs/reward_manager"
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
	pool := database.CreateTestDatabase(t, "test_solana_indexer_program")
	rpcClient := fake_rpc_client.FakeRpcClient{}
	logger := zap.NewNop()

	indexer := New(common.GrpcConfig{}, &rpcClient, pool, config.Cfg, nil, logger)

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

func TestHandleUpdate_ClaimableInit(t *testing.T) {
	// Deps
	pool := database.CreateTestDatabase(t, "test_solana_indexer_program")
	rpcClient := fake_rpc_client.FakeRpcClient{}
	transactionCache, err := otter.MustBuilder[solana.Signature, *rpc.GetTransactionResult](10).Build()
	require.NoError(t, err, "failed to create cache")
	logger := zap.NewNop()

	// Expected results
	txSig := solana.MustSignatureFromBase58("4WnUo5yDzE8yT82VzRwg9N5UC5sSXRPaaKSeQhfLSRHSCCfufpLWeg4x14dCSdAy66A5aV4ewv4KMc6fXEKwWF3m")
	account := "C9v6wyJRwtwKiGsozxAykkk2jkVnxGf7buzkXhfGvApY"
	mint := "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v"
	ethereumAddress := "0xd52503493a1fe2d9b3dfcf412337828a97bc196f"
	slot := uint64(373483258)

	// Load fixture (taken from real transaction on production)
	txJson, err := os.ReadFile("./fixtures/claimable_tokens_init_transaction_test_fixture.json")
	require.NoError(t, err, "failed to read transaction test fixture")

	var txResult rpc.GetTransactionResult
	err = json.Unmarshal(txJson, &txResult)
	require.NoError(t, err, "failed to unmarshal transaction test fixture")

	// Fixture uses production claimable tokens program ID
	claimable_tokens.SetProgramID(solana.MustPublicKeyFromBase58("Ewkv3JahEFRKkcJmpoKB7pXbnUHwjAyXiwEo4ZY2rezQ"))

	// Put the transaction in the cache so that the indexer loads it from there
	transactionCache.Set(txSig, &txResult)

	// Create the update
	update := pb.SubscribeUpdate{
		UpdateOneof: &pb.SubscribeUpdate_Transaction{
			Transaction: &pb.SubscribeUpdateTransaction{
				Transaction: &pb.SubscribeUpdateTransactionInfo{
					Signature: txSig[:],
				},
			},
		},
	}

	// Run the test
	indexer := New(common.GrpcConfig{}, &rpcClient, pool, config.Cfg, &transactionCache, logger)
	err = indexer.HandleUpdate(t.Context(), &update)
	require.NoError(t, err, "failed to handle update")

	// Check the claimable account was inserted
	var exists bool
	sql := `
		SELECT EXISTS (
			SELECT 1
			FROM sol_claimable_accounts
			WHERE signature = @signature
				AND instruction_index = @instruction_index
				AND slot = @slot
				AND mint = @mint
				AND ethereum_address = @ethereum_address
				AND account = @account
			LIMIT 1
		)
	`
	err = pool.QueryRow(t.Context(), sql, pgx.NamedArgs{
		"signature":         txSig.String(),
		"instruction_index": 0,
		"slot":              slot,
		"mint":              mint,
		"ethereum_address":  ethereumAddress,
		"account":           account,
	}).Scan(&exists)
	require.NoError(t, err)
	assert.True(t, exists, "claimable account should exist")
}

func TestHandleUpdate_ClaimableTransfer(t *testing.T) {
	// Deps
	pool := database.CreateTestDatabase(t, "test_solana_indexer_program")
	rpcClient := fake_rpc_client.FakeRpcClient{}
	transactionCache, err := otter.MustBuilder[solana.Signature, *rpc.GetTransactionResult](10).Build()
	require.NoError(t, err, "failed to create cache")
	logger := zap.NewNop()

	// Expected results
	txSig := solana.MustSignatureFromBase58("267Sv7Ub29fVDZ7a2ah326WwZDi5FfitiAwJ8Dvx6TKnBwRzZNELT9zf1LCrFNHj8sT88PJGx7yFQaLUw2AcKnDV")
	amount := int64(100000000)
	fromAccount := "9y3ZhQkFCSy323oahK9LW9h4yKoGjWSSomqwN9QsTb6C"
	toAccount := "5V9wSDJKbhpjAj9FE1GWZyNqgoDSroUXJMDtNZzWHeWo"
	senderEthAddress := "0x3ced9f71aa6e8a20279a2f932d673db609f5a247"
	slot := uint64(373475076)

	// Load fixture (taken from real transaction on production)
	txJson, err := os.ReadFile("./fixtures/claimable_tokens_transfer_transaction_test_fixture.json")
	require.NoError(t, err, "failed to read transaction test fixture")

	var txResult rpc.GetTransactionResult
	err = json.Unmarshal(txJson, &txResult)
	require.NoError(t, err, "failed to unmarshal transaction test fixture")

	// Fixture uses production claimable tokens program ID
	claimable_tokens.SetProgramID(solana.MustPublicKeyFromBase58("Ewkv3JahEFRKkcJmpoKB7pXbnUHwjAyXiwEo4ZY2rezQ"))

	// Put the transaction in the cache so that the indexer loads it from there
	transactionCache.Set(txSig, &txResult)

	// Create the update
	update := pb.SubscribeUpdate{
		UpdateOneof: &pb.SubscribeUpdate_Transaction{
			Transaction: &pb.SubscribeUpdateTransaction{
				Transaction: &pb.SubscribeUpdateTransactionInfo{
					Signature: txSig[:],
				},
			},
		},
	}

	// Run the test
	indexer := New(common.GrpcConfig{}, &rpcClient, pool, config.Cfg, &transactionCache, logger)
	err = indexer.HandleUpdate(t.Context(), &update)
	require.NoError(t, err, "failed to handle update")

	// Check the claimable account transfer was inserted
	var exists bool
	sql := `
		SELECT EXISTS (
			SELECT 1
			FROM sol_claimable_account_transfers
			WHERE signature = @signature
				AND instruction_index = @instruction_index
				AND amount = @amount
				AND slot = @slot
				AND from_account = @from_account
				AND to_account = @to_account
				AND sender_eth_address = @sender_eth_address
			LIMIT 1
		)
	`
	err = pool.QueryRow(t.Context(), sql, pgx.NamedArgs{
		"signature":          txSig.String(),
		"instruction_index":  1,
		"slot":               slot,
		"amount":             amount,
		"from_account":       fromAccount,
		"to_account":         toAccount,
		"sender_eth_address": senderEthAddress,
	}).Scan(&exists)
	require.NoError(t, err)
	assert.True(t, exists, "claimable account transfer should exist")
}

func TestHandleUpdate_RewardDisbursement(t *testing.T) {
	// Deps
	pool := database.CreateTestDatabase(t, "test_solana_indexer_program")
	rpcClient := fake_rpc_client.FakeRpcClient{}
	transactionCache, err := otter.MustBuilder[solana.Signature, *rpc.GetTransactionResult](10).Build()
	require.NoError(t, err, "failed to create cache")
	logger := zap.NewNop()

	// Expected results
	txSig := solana.MustSignatureFromBase58("474H2JPrzyHgBxypz9K9V2DyZjo8ha7FhSASyc2WYseMFTJtgaBjuEaVx4p59hwVh53V1CYmEJNVUxcG1PbnTvJT")
	amount := int64(100000000)
	challengeID := "e"
	specifier := "29d545402025101505"
	userBank := "45UesTt1A8zG6ZjKPJnigNU53DzVuoKpRCV5bKoQNEtZ"
	recipientEthAddress := "0xdffde5a630da4d07c988c668b2c929637096d96d"
	slot := uint64(373471928)

	// Load fixture (taken from real transaction on production)
	txJson, err := os.ReadFile("./fixtures/reward_manager_evaluate_transaction_test_fixture.json")
	require.NoError(t, err, "failed to read transaction test fixture")

	var txResult rpc.GetTransactionResult
	err = json.Unmarshal(txJson, &txResult)
	require.NoError(t, err, "failed to unmarshal transaction test fixture")

	// Fixture uses production reward manager program ID
	reward_manager.SetProgramID(solana.MustPublicKeyFromBase58("DDZDcYdQFEMwcu2Mwo75yGFjJ1mUQyyXLWzhZLEVFcei"))

	// Put the transaction in the cache so that the indexer loads it from there
	transactionCache.Set(txSig, &txResult)

	// Create the update
	update := pb.SubscribeUpdate{
		UpdateOneof: &pb.SubscribeUpdate_Transaction{
			Transaction: &pb.SubscribeUpdateTransaction{
				Transaction: &pb.SubscribeUpdateTransactionInfo{
					Signature: txSig[:],
				},
			},
		},
	}

	// Run the test
	indexer := New(common.GrpcConfig{}, &rpcClient, pool, config.Cfg, &transactionCache, logger)
	err = indexer.HandleUpdate(t.Context(), &update)
	require.NoError(t, err, "failed to handle update")

	// Check the claimable account transfer was inserted
	var exists bool
	sql := `
		SELECT EXISTS (
			SELECT 1
			FROM sol_reward_disbursements
			WHERE signature = @signature
				AND instruction_index = @instruction_index
				AND amount = @amount
				AND slot = @slot
				AND user_bank = @user_bank
				AND challenge_id = @challenge_id
				AND specifier = @specifier
				AND recipient_eth_address = @recipient_eth_address
			LIMIT 1
		)
	`
	err = pool.QueryRow(t.Context(), sql, pgx.NamedArgs{
		"signature":             txSig.String(),
		"instruction_index":     0,
		"slot":                  slot,
		"amount":                amount,
		"user_bank":             userBank,
		"challenge_id":          challengeID,
		"specifier":             specifier,
		"recipient_eth_address": recipientEthAddress,
	}).Scan(&exists)
	require.NoError(t, err)
	assert.True(t, exists, "reward disbursement should exist")
}

func TestHandleUpdate_PaymentRouterPurchase(t *testing.T) {
	// Deps
	pool := database.CreateTestDatabase(t, "test_solana_indexer_program")
	rpcClient := fake_rpc_client.FakeRpcClient{}
	transactionCache, err := otter.MustBuilder[solana.Signature, *rpc.GetTransactionResult](10).Build()
	require.NoError(t, err, "failed to create cache")
	logger := zap.NewNop()

	// Expected results
	txSig := solana.MustSignatureFromBase58("4cDX7FuWB9tZgfqaNiYjPjY2pxcWUinv5PCHhize2F73xRNqiomCBmwuxZMm1Ja9ueyaRjVBUVfgrJ3s2yFBJpq5")
	fromAccount := "7YRsw96JjbLKfXY51c64kSvTK8opgxw292GT8J1HGKf3"
	amount := int64(15000000)
	contentType := "track"
	contentID := 194382587
	validAfterBlocknumber := 106452905
	buyerUserID := 474820902
	accessType := "stream"
	slot := uint64(373232547)
	city := "Newark"
	region := "Texas"
	country := "United States"
	receiver := "8rdZD9XgrxxTZmK2GQaKnSUnZYfvEPFuMpzHjo3dL4Wp"
	receiverAmount := uint64(13500000)
	network := "7vGA3fcjvxa3A11MAxmyhFtYowPLLCNyvoxxgN3NN2Vf"
	networkAmount := uint64(1500000)

	// Load fixture (taken from real transaction on production)
	txJson, err := os.ReadFile("./fixtures/payment_router_purchase_transaction_test_fixture.json")
	require.NoError(t, err, "failed to read transaction test fixture")

	var txResult rpc.GetTransactionResult
	err = json.Unmarshal(txJson, &txResult)
	require.NoError(t, err, "failed to unmarshal transaction test fixture")

	// Fixture uses production reward manager program ID
	payment_router.SetProgramID(solana.MustPublicKeyFromBase58("paytYpX3LPN98TAeen6bFFeraGSuWnomZmCXjAsoqPa"))

	// Put the transaction in the cache so that the indexer loads it from there
	transactionCache.Set(txSig, &txResult)

	// Create the update
	update := pb.SubscribeUpdate{
		UpdateOneof: &pb.SubscribeUpdate_Transaction{
			Transaction: &pb.SubscribeUpdateTransaction{
				Transaction: &pb.SubscribeUpdateTransactionInfo{
					Signature: txSig[:],
				},
			},
		},
	}

	// Run the test
	indexer := New(common.GrpcConfig{}, &rpcClient, pool, config.Cfg, &transactionCache, logger)
	err = indexer.HandleUpdate(t.Context(), &update)
	require.NoError(t, err, "failed to handle update")

	// Check the purchase was inserted
	var exists bool
	sql := `
		SELECT EXISTS (
			SELECT 1
			FROM sol_purchases
			WHERE signature = @signature
				AND instruction_index = @instruction_index
				AND amount = @amount
				AND slot = @slot
				AND from_account = @from_account
				AND content_type = @content_type
				AND content_id = @content_id
				AND buyer_user_id = @buyer_user_id
				AND access_type = @access_type
				AND valid_after_blocknumber = @valid_after_blocknumber
				AND city = @city
				AND region = @region
				AND country = @country
			LIMIT 1
		)
	`
	err = pool.QueryRow(t.Context(), sql, pgx.NamedArgs{
		"signature":               txSig.String(),
		"instruction_index":       4,
		"slot":                    slot,
		"amount":                  amount,
		"from_account":            fromAccount,
		"content_type":            contentType,
		"content_id":              contentID,
		"buyer_user_id":           buyerUserID,
		"access_type":             accessType,
		"valid_after_blocknumber": validAfterBlocknumber,
		"city":                    city,
		"region":                  region,
		"country":                 country,
	}).Scan(&exists)
	require.NoError(t, err)
	assert.True(t, exists, "purchase should exist")

	// Check the payments were inserted
	sql = `
		SELECT EXISTS (
			SELECT 1
			FROM sol_payments
			WHERE signature = @signature
				AND instruction_index = @instruction_index
				AND amount = @amount
				AND slot = @slot
				AND route_index = @route_index
				AND to_account = @to_account
			LIMIT 1
		)
	`

	err = pool.QueryRow(t.Context(), sql, pgx.NamedArgs{
		"signature":         txSig.String(),
		"instruction_index": 4,
		"slot":              slot,
		"amount":            receiverAmount,
		"route_index":       0,
		"to_account":        receiver,
	}).Scan(&exists)
	require.NoError(t, err)
	assert.True(t, exists, "receiver payment should exist")

	err = pool.QueryRow(t.Context(), sql, pgx.NamedArgs{
		"signature":         txSig.String(),
		"instruction_index": 4,
		"slot":              slot,
		"amount":            networkAmount,
		"route_index":       1,
		"to_account":        network,
	}).Scan(&exists)
	require.NoError(t, err)
	assert.True(t, exists, "network payment should exist")
}

func TestHandleUpdate_RewardInit(t *testing.T) {
	// Deps
	pool := database.CreateTestDatabase(t, "test_solana_indexer_program")
	rpcClient := fake_rpc_client.FakeRpcClient{}
	transactionCache, err := otter.MustBuilder[solana.Signature, *rpc.GetTransactionResult](10).Build()
	require.NoError(t, err, "failed to create cache")
	logger := zap.NewNop()

	// Expected results
	txSig := solana.MustSignatureFromBase58("yiuyxK1b46frtiwyXxYjdQiYjWTCseAzdQ9v1QBYauSFVES3Rys2ra1oL5LpEtrNj7iXBEWnnzexNVaLvQtfb5J")

	// Load fixture (taken from real transaction on production)
	txJson, err := os.ReadFile("./fixtures/reward_manager_init_transaction_test_fixture.json")
	require.NoError(t, err, "failed to read transaction test fixture")

	var txResult rpc.GetTransactionResult
	err = json.Unmarshal(txJson, &txResult)
	require.NoError(t, err, "failed to unmarshal transaction test fixture")

	// Fixture uses production reward manager program ID
	reward_manager.SetProgramID(solana.MustPublicKeyFromBase58("DDZDcYdQFEMwcu2Mwo75yGFjJ1mUQyyXLWzhZLEVFcei"))

	// Put the transaction in the cache so that the indexer loads it from there
	transactionCache.Set(txSig, &txResult)

	// Create the update
	update := pb.SubscribeUpdate{
		UpdateOneof: &pb.SubscribeUpdate_Transaction{
			Transaction: &pb.SubscribeUpdateTransaction{
				Transaction: &pb.SubscribeUpdateTransactionInfo{
					Signature: txSig[:],
				},
			},
		},
	}

	// Run the test
	indexer := New(common.GrpcConfig{}, &rpcClient, pool, config.Cfg, &transactionCache, logger)
	err = indexer.HandleUpdate(t.Context(), &update)
	require.NoError(t, err, "failed to handle update")

	// Check the reward manager init was inserted
	var exists bool
	sql := `
		SELECT EXISTS (
			SELECT 1
			FROM sol_reward_manager_inits
			WHERE signature = @signature
				AND instruction_index = @instruction_index
				AND slot = @slot
				AND min_votes = @min_votes
				AND reward_manager_state = @reward_manager_state
				AND token_source = @token_source
				AND mint = @mint
				AND manager = @manager
				AND authority = @authority
			LIMIT 1
		)
	`
	err = pool.QueryRow(t.Context(), sql, pgx.NamedArgs{
		"signature":            txSig.String(),
		"instruction_index":    2,
		"slot":                 uint64(375202112),
		"min_votes":            uint8(3),
		"reward_manager_state": "6mrvbuCnBir7VeD3Vr9zdL8McwfLCT4Xh8mTMGukfqUs",
		"token_source":         "CoS8SobE44wGq8CGi2BekLDih7GhSKgjszhTyitfGMPW",
		"mint":                 "c57yR7PxEWhHw72BstEZv5w28RkA7E1eUjSLd8vxMP3",
		"manager":              "GrFKAqUtubUMG4xm7hHqPctBjDxA3MNCBQqNPYNr1Se4",
		"authority":            "CK468ayRnucvCaB1s9vUN3ue7VhnPqSQRFmceHeqPCRE",
	}).Scan(&exists)
	require.NoError(t, err)
	assert.True(t, exists, "reward manager init should exist")
}
