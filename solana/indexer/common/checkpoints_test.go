package common

import (
	"testing"

	"api.audius.co/database"
	"github.com/rpcpool/yellowstone-grpc/examples/golang/proto"
	"github.com/test-go/testify/assert"
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
