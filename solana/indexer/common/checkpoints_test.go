package common

import (
	"context"
	"testing"

	"api.audius.co/database"
	"api.audius.co/solana/indexer/fake_rpc_client"
	"github.com/gagliardetto/solana-go/rpc"
	"github.com/rpcpool/yellowstone-grpc/examples/golang/proto"
	pb "github.com/rpcpool/yellowstone-grpc/examples/golang/proto"
	"github.com/stretchr/testify/require"
	"github.com/test-go/testify/assert"
	"go.uber.org/zap"
)

func TestCheckpoints(t *testing.T) {
	pool := database.CreateTestDatabase(t, "test_solana_indexer_common")
	defer pool.Close()

	req := proto.SubscribeRequest{}
	id, err := insertCheckpointStart(t.Context(), pool, "backfill", 100, &req)
	assert.NoError(t, err, "failed to insert checkpoint start")
	assert.NotEmpty(t, id, "checkpoint ID should not be empty")

	err = UpdateCheckpoint(t.Context(), pool, id, 201)
	assert.NoError(t, err, "failed to update checkpoint")

	slot, err := GetCheckpointSlot(t.Context(), pool, "backfill", &req)
	assert.NoError(t, err, "failed to get checkpoint slot")
	assert.Equal(t, uint64(201), slot, "checkpoint slot should match updated value")

	id2, err := InsertBackfillCheckpoint(t.Context(), pool, 100, 200, "foo")
	assert.NoError(t, err, "failed to insert backfill checkpoint")
	assert.NotEmpty(t, id2, "backfill checkpoint ID should not be empty")
}

func TestEnsureCheckpoints(t *testing.T) {
	pool := database.CreateTestDatabase(t, "test_solana_indexer_common")
	defer pool.Close()

	// Setup mock RPC clients
	rpcClientInit := &fake_rpc_client.FakeRpcClient{
		GetSlotFunc: func(ctx context.Context, commitment rpc.CommitmentType) (uint64, error) {
			return 200, nil
		},
	}

	rpcClientWithinMax := &fake_rpc_client.FakeRpcClient{
		GetSlotFunc: func(ctx context.Context, commitment rpc.CommitmentType) (uint64, error) {
			return 2500, nil
		},
	}

	rpcClientBeyondMax := &fake_rpc_client.FakeRpcClient{
		GetSlotFunc: func(ctx context.Context, commitment rpc.CommitmentType) (uint64, error) {
			return 5000, nil
		},
	}

	logger := zap.NewNop()
	commitment := pb.CommitmentLevel_CONFIRMED
	req := proto.SubscribeRequest{
		Commitment: &commitment,
	}
	checkpointId, slot, err := EnsureCheckpoint(t.Context(), "backfill", pool, rpcClientInit, &req, logger)
	require.NoError(t, err, "failed to ensure checkpoint")
	assert.NotEmpty(t, checkpointId, "checkpoint ID should not be empty")
	assert.Equal(t, uint64(100), slot, "slot should be current slot - 100 for new subscription")
	initialId := checkpointId

	checkpointId, slot, err = EnsureCheckpoint(t.Context(), "backfill", pool, rpcClientWithinMax, &req, logger)
	require.NoError(t, err, "failed to ensure checkpoint")
	assert.NotEmpty(t, checkpointId, "checkpoint ID should not be empty")
	assert.Equal(t, uint64(100), slot, "slot should be last indexed slot")

	checkpointId, slot, err = EnsureCheckpoint(t.Context(), "backfill", pool, rpcClientBeyondMax, &req, logger)
	require.NoError(t, err, "failed to ensure checkpoint")
	assert.NotEmpty(t, checkpointId, "checkpoint ID should not be empty")
	assert.NotEqual(t, initialId, checkpointId, "checkpoint ID should be new")
	assert.Equal(t, uint64(2500), slot, "slot should be current slot - MAX_SLOT_GAP")

}
