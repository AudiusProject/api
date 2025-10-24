package dbc

import (
	"encoding/base64"
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

	err = indexer.HandleUpdate(t.Context(), &update)
	require.NoError(t, err)

	slot, err := common.GetCheckpointSlot(t.Context(), pool, "test", &request)
	require.NoError(t, err)
	assert.Equal(t, expectedSlot, slot, "checkpoint slot should be updated")
}

func TestHandleUpdate_Migration(t *testing.T) {
	pool := database.CreateTestDatabase(t, "test_solana_indexer_damm_v2")
	rpcClient := fake_rpc_client.FakeRpcClient{}
	transactionCache, err := otter.MustBuilder[solana.Signature, *rpc.GetTransactionResult](10).Build()
	require.NoError(t, err, "failed to create cache")
	logger := zap.NewNop()

	// Add artist coin
	_, err = pool.Exec(t.Context(), `
		INSERT INTO artist_coins (mint, ticker, name, decimals, user_id)
		VALUES ('bearR26zyyB3fNQm5wWv1ZfN8MPQDUMwaAuoG79b1Yj', 'BEAR', 'Bear', 9, 0)
	`)
	require.NoError(t, err, "failed to insert artist coin")

	// Fetched using RPC call and copy/pasted the result
	respJsonBytes, err := os.ReadFile("./migration_transaction_test_fixture.json")
	require.NoError(t, err)
	respJson := string(respJsonBytes)

	var resp rpc.GetTransactionResult
	err = json.Unmarshal([]byte(respJson), &resp)
	require.NoError(t, err)

	txSig := solana.MustSignatureFromBase58("93takW7UMBsJgGNH9oARpTT5EiEtJ7c2u6PCzHAsFMQ6P2Sejy5zJpn4sAaxMLHcfLPvMtFE87piofkH22oxuFz")

	transactionCache.Set(txSig, &resp)

	poolAddress := solana.MustPublicKeyFromBase58("J5LCsaaCWcYmzes8qwKmg89zzEtnbYkxFxD9YRU5auPY")
	poolBase64 := "1eAF0WJFd1wAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAG9Tc1jgMJCyOX1vksZXFU3IYphR0oodY0slRsdoUdgKrMKOzc0mg2NPexupQDHoGcVoHifYtOmHGEwAVsO0Z3QjgPlTbLz1Ps0B4mY1IShFnTtqdLRHMu8x62whqE/2AvYhAP2AbR4EbLQJaWf2CK3epbbZGBxR+R3zD54jzqne4Uh2rsFMELtnbPjkj1rWBozcMGqiGiqiysf2F5cey7qsvh70GiWgKlc9OkIgPAAAAAAAAAAAAALvd+QoIAAAAAAAAAAAAAAAAAAAAAAAAAGCPwvUoXI8CAAAAAAAAAAC7W7cVAAAAAAABAAADAAAAAAAAAAAAAAC73fkKCAAAAAAAAAAAAAAABXfnKyAAAADP/cloAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=="
	poolData, err := base64.StdEncoding.DecodeString(poolBase64)
	require.NoError(t, err)

	update := pb.SubscribeUpdate{
		UpdateOneof: &pb.SubscribeUpdate_Account{
			Account: &pb.SubscribeUpdateAccount{
				Slot: 367312008,
				Account: &pb.SubscribeUpdateAccountInfo{
					Pubkey:       poolAddress.Bytes(),
					Data:         poolData,
					TxnSignature: txSig[:],
				},
			},
		},
	}

	indexer := New(common.GrpcConfig{}, &rpcClient, pool, &transactionCache, logger)
	err = indexer.HandleUpdate(t.Context(), &update)
	require.NoError(t, err)

	// Verify that the dbc pool was inserted
	sql := `
		SELECT EXISTS (
			SELECT 1
			FROM sol_meteora_dbc_pools
			WHERE account = @account
				AND slot = @slot
				AND config = @config
				AND creator = @creator
				AND base_mint = @base_mint
				AND base_vault = @base_vault
				AND quote_vault = @quote_vault
				AND base_reserve = @base_reserve
				AND quote_reserve = @quote_reserve
				AND protocol_base_fee = @protocol_base_fee
				AND partner_base_fee = @partner_base_fee
				AND partner_quote_fee = @partner_quote_fee
				AND sqrt_price = @sqrt_price
				AND activation_point = @activation_point
				AND pool_type = @pool_type
				AND is_migrated = @is_migrated
				AND is_partner_withdraw_surplus = @is_partner_withdraw_surplus
				AND is_protocol_withdraw_surplus = @is_protocol_withdraw_surplus
				AND migration_progress = @migration_progress
				AND is_withdraw_leftover = @is_withdraw_leftover
				AND is_creator_withdraw_surplus = @is_creator_withdraw_surplus
				AND migration_fee_withdraw_status = @migration_fee_withdraw_status
				AND finish_curve_timestamp = @finish_curve_timestamp
				AND creator_base_fee = @creator_base_fee
				AND creator_quote_fee = @creator_quote_fee
			LIMIT 1
		)
	`
	var exists bool
	err = pool.QueryRow(t.Context(), sql, pgx.NamedArgs{
		"account":                       poolAddress.String(),
		"slot":                          int64(367312008),
		"config":                        "2seGMFauXC22DX8hbop1gh54W1uW8YREWhsU7JuCptTj",
		"creator":                       "CXFjXpXVQqv4jy5bjauLvECrDsKESQB6BPkSiC6dAmak",
		"base_mint":                     "bearR26zyyB3fNQm5wWv1ZfN8MPQDUMwaAuoG79b1Yj",
		"base_vault":                    "DkrZF8DT18gu4n9Q26c46r49ciJwNt8S8jbQhouEEJnA",
		"quote_vault":                   "DQWe8KbbGSUyESbbfzG9keBmhtjMRbRVg4e3wH2h1KFT",
		"base_reserve":                  uint64(749999999999750059),
		"quote_reserve":                 uint64(17079211052949),
		"protocol_base_fee":             uint64(0),
		"protocol_quote_fee":            uint64(34543885755),
		"partner_base_fee":              uint64(0),
		"partner_quote_fee":             uint64(0),
		"sqrt_price":                    uint64(184467440737095520),
		"activation_point":              int64(364338107),
		"pool_type":                     uint8(0),
		"is_migrated":                   uint8(1),
		"is_partner_withdraw_surplus":   uint8(0),
		"is_protocol_withdraw_surplus":  uint8(0),
		"migration_progress":            3,
		"is_withdraw_leftover":          uint8(0),
		"is_creator_withdraw_surplus":   uint8(0),
		"migration_fee_withdraw_status": uint8(0),
		"finish_curve_timestamp":        int64(1758068175),
		"creator_base_fee":              uint64(0),
		"creator_quote_fee":             uint64(0),
	}).Scan(&exists)
	require.NoError(t, err, "failed to query for dbc pool")
	assert.True(t, exists, "dbc pool should exist after migration")

	// Verify the migration instruction was inserted
	sql = `
		SELECT EXISTS (
			SELECT 1
			FROM sol_meteora_dbc_migrations
			WHERE signature = @signature
				AND instruction_index = @instruction_index
				AND slot = @slot
				AND dbc_pool = @dbc_pool
				AND migration_metadata = @migration_metadata
				AND config = @config
				AND dbc_pool_authority = @dbc_pool_authority
				AND damm_v2_pool = @damm_v2_pool
				AND first_position_nft_mint = @first_position_nft_mint
				AND first_position_nft_account = @first_position_nft_account
				AND first_position = @first_position
				AND second_position_nft_mint = @second_position_nft_mint
				AND second_position_nft_account = @second_position_nft_account
				AND second_position = @second_position
				AND damm_pool_authority = @damm_pool_authority
				AND base_mint = @base_mint
				AND quote_mint = @quote_mint
			LIMIT 1
		)
	`
	err = pool.QueryRow(t.Context(), sql, pgx.NamedArgs{
		"signature":                   txSig.String(),
		"instruction_index":           int64(0),
		"slot":                        int64(367312008),
		"dbc_pool":                    poolAddress.String(),
		"migration_metadata":          "BFjrGaLwPznyjCQ47dbP8xVy1QLjdJ7w8CpwZ4sZwqpf",
		"config":                      "2seGMFauXC22DX8hbop1gh54W1uW8YREWhsU7JuCptTj",
		"dbc_pool_authority":          "FhVo3mqL8PW5pH5U2CN4XE33DokiyZnUwuGpH2hmHLuM",
		"damm_v2_pool":                "9avVRGRvPsSYiXKBMHnC6RNPbwN5yE3v7fD8FibgScwA",
		"first_position_nft_mint":     "DgWF4Huj8a4yyGBPaAw4kqQFBuD5BujbHN7VKWaEBp4t",
		"first_position_nft_account":  "CL5nkYmVBNHRDZLZovyK6RGbv2SFjJRfPgf4pFknitBn",
		"first_position":              "GRYndBmh1aoQjMXQm4NPs7oQ288o6xghrWVXytyM2c1Q",
		"second_position_nft_mint":    "76uQDWksWWvFKHn4ZNPTpR9jJ9x3eqgsmcHdb8FmXS3o",
		"second_position_nft_account": "GXKmP3hBvRnor2ieNVrcAVGgDev1HUYNrwjZ4yJDwtU5",
		"second_position":             "9jUTYHEypL6XwYRSk5CZaKMEmNuQoRaPM17JDaNpk6G7",
		"damm_pool_authority":         "HLnpSz9h2S4hiLQ43rnSD9XkcUThA7B8hQMKmDaiTLcC",
		"base_mint":                   "bearR26zyyB3fNQm5wWv1ZfN8MPQDUMwaAuoG79b1Yj",
		"quote_mint":                  "9LzCMqDgTKYz9Drzqnpgee3SGa89up3a247ypMj2xrqM",
	}).Scan(&exists)
	require.NoError(t, err, "failed to query for dbc migration")
	assert.True(t, exists, "dbc migration should exist after indexing")

	sql = `
		SELECT EXISTS (
			SELECT 1
			FROM artist_coins
			WHERE mint = @mint AND damm_v2_pool = @damm_v2_pool
			LIMIT 1
		)
	`
	err = pool.QueryRow(t.Context(), sql, pgx.NamedArgs{
		"mint":         "bearR26zyyB3fNQm5wWv1ZfN8MPQDUMwaAuoG79b1Yj",
		"damm_v2_pool": "9avVRGRvPsSYiXKBMHnC6RNPbwN5yE3v7fD8FibgScwA",
	}).Scan(&exists)
	require.NoError(t, err, "failed to query for artist coin update")
	assert.True(t, exists, "artist coin should be updated with damm v2 pool")
}
