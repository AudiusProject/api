package damm_v2

import (
	"encoding/base64"
	"testing"

	"api.audius.co/database"
	"api.audius.co/solana/indexer/common"
	"api.audius.co/solana/indexer/fake_rpc_client"
	"github.com/gagliardetto/solana-go"
	pb "github.com/rpcpool/yellowstone-grpc/examples/golang/proto"
	"github.com/test-go/testify/assert"
	"github.com/test-go/testify/require"
	"go.uber.org/zap"
)

func TestHandleUpdate_SlotCheckpoint(t *testing.T) {
	pool := database.CreateTestDatabase(t, "test_solana_indexer_damm_v2")
	rpcClient := fake_rpc_client.FakeRpcClient{}
	logger := zap.NewNop()

	indexer := New(common.GrpcConfig{}, &rpcClient, pool, logger)

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

func TestHandleUpdate_DammV2PoolUpdate(t *testing.T) {
	pool := database.CreateTestDatabase(t, "test_solana_indexer_damm_v2")
	rpcClient := fake_rpc_client.FakeRpcClient{}
	logger := zap.NewNop()

	indexer := New(common.GrpcConfig{}, &rpcClient, pool, logger)

	// From real on-chain account data
	address := solana.MustPublicKeyFromBase58("D9iJqMbgQJLFt5PAAiTJTMNsMAMueukzoe1EK2r1g3WH")
	poolBase64 := "8ZptBBGxbbyAlpgAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAFAAUAAAAAAABAAAAAAAAAGCk3AC8AwAAAQAKAHgAiBPGROhoAAAAAMsQx7q4jQYAAAAAAAAAAAChIqYBNRzVAQAAAAAAAAAA4CICAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAACOkzYTTyijBQphnsA7NYukXXDff56Bp/GJdn5GamlMZ7/DPMLnXBSHbMN5KDkE9JB3ZpESJXuzrf82mLYCJJQHm/3HSrkp1wPzAbe6y0uFypnr4Yeci2kPU8TWr9TEmTY/2aZknQDPJED5N2M3ytBL5gl4lD8TdKznaJkMDHT44AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAANpjaB9yhrzIBnGeLCtQogFXJDv8lmixFSC8U4Q+3NsISFBg5M0NQHAAaMJHGBEGAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAFiinY8UAAAAAAAAAAAAAAAAAAAAAAAAAFA7AQABAAAAAAAAAAAAAACbV2lOqRpchLHE/v8AAAAAIiTN1Ql11QEAAAAAAAAAAIhx5mgAAAAAAQAAAAEAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAJE91kBWow0AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAASFBg5M0NQHAAaMJHGBEGAAAAAAAAAAAAAAAAAAAAAAASYim9UgAAAAAAAAAAAAAAAAAAAAAAAABYop2PFAAAAAAAAAAAAAAAAAAAAAAAAAACAAAAAAAAAAAAAAAAAAAA2mNoH3KGvMgGcZ4sK1CiAVckO/yWaLEVILxThD7c2wgAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA="
	poolData, err := base64.StdEncoding.DecodeString(poolBase64)
	require.NoError(t, err)

	update := pb.SubscribeUpdate{
		Filters: []string{NAME},
		UpdateOneof: &pb.SubscribeUpdate_Account{
			Account: &pb.SubscribeUpdateAccount{
				Account: &pb.SubscribeUpdateAccountInfo{
					Pubkey: address.Bytes(),
					Data:   poolData,
				},
			},
		},
	}

	err = indexer.HandleUpdate(t.Context(), &update)
	require.NoError(t, err)

	rows, err := pool.Query(t.Context(), `
		SELECT EXISTS (
			SELECT 1
			FROM sol_meteora_damm_v2_pools
			JOIN sol_meteora_damm_v2_pool_metrics ON sol_meteora_damm_v2_pool_metrics.pool = sol_meteora_damm_v2_pools.account
			JOIN sol_meteora_damm_v2_pool_fees ON sol_meteora_damm_v2_pool_fees.pool = sol_meteora_damm_v2_pools.account
			JOIN sol_meteora_damm_v2_pool_base_fees ON sol_meteora_damm_v2_pool_base_fees.pool = sol_meteora_damm_v2_pools.account
			JOIN sol_meteora_damm_v2_pool_dynamic_fees ON sol_meteora_damm_v2_pool_dynamic_fees.pool = sol_meteora_damm_v2_pools.account
			LIMIT 1
		)
	`)
	require.NoError(t, err)
	defer rows.Close()
}

func TestHandleUpdate_DammV2PositionUpdate(t *testing.T) {
	pool := database.CreateTestDatabase(t, "test_solana_indexer_damm_v2")
	rpcClient := fake_rpc_client.FakeRpcClient{}
	logger := zap.NewNop()

	indexer := New(common.GrpcConfig{}, &rpcClient, pool, logger)

	// From real on-chain account data
	address := solana.MustPublicKeyFromBase58("5bYLydDXt1K5zroychcbrVbhGRUpheXdq5w41uccazPB")
	poolBase64 := "qryP5HpA99C0h5iaMb9or5qzYmaPKH7cBpP1GTyw5pa9SMlEQMuk4oeLsnqCTyioPLOFt664lEHr2woSYFq4Z3N6xFLWwGDSAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAADUszHGm5oNAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAACQoMPLmBiA4ADThI4wIAwAAAAAAAAAAABGmGkQpAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
	poolData, err := base64.StdEncoding.DecodeString(poolBase64)
	require.NoError(t, err)

	update := pb.SubscribeUpdate{
		Filters: []string{address.String()},
		UpdateOneof: &pb.SubscribeUpdate_Account{
			Account: &pb.SubscribeUpdateAccount{
				Account: &pb.SubscribeUpdateAccountInfo{
					Pubkey: address.Bytes(),
					Data:   poolData,
				},
			},
		},
	}

	err = indexer.HandleUpdate(t.Context(), &update)
	require.NoError(t, err)

	rows, err := pool.Query(t.Context(), `
		SELECT EXISTS (
			SELECT 1
			FROM sol_meteora_damm_v2_positions
			JOIN sol_meteora_damm_v2_position_metrics ON sol_meteora_damm_v2_position_metrics.position = sol_meteora_damm_v2_positions.account
			LIMIT 1
		)
	`)
	require.NoError(t, err)
	defer rows.Close()
}
