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
		INSERT INTO artist_coins (mint, ticker, name, decimals, user_id, dbc_pool)
		VALUES ('bearR26zyyB3fNQm5wWv1ZfN8MPQDUMwaAuoG79b1Yj', 'BEAR', 'Bear', 9, 0, 'J5LCsaaCWcYmzes8qwKmg89zzEtnbYkxFxD9YRU5auPY')
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

	sql := `
		SELECT EXISTS (
			SELECT 1
			FROM artist_coins
			JOIN sol_meteora_dbc_pools ON sol_meteora_dbc_pools.account = artist_coins.dbc_pool
			JOIN sol_meteora_dbc_migrations ON sol_meteora_dbc_migrations.dbc_pool = sol_meteora_dbc_pools.account		
			WHERE artist_coins.damm_v2_pool IS NOT NULL
			LIMIT 1
		)
	`
	var exists bool
	err = pool.QueryRow(t.Context(), sql).Scan(&exists)
	require.NoError(t, err, "failed to query for dbc pool")
	assert.True(t, exists, "damm v2 pool should exist after migration")
}
