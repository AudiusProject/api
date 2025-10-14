package common

import (
	"testing"

	"api.audius.co/database"
	pb "github.com/rpcpool/yellowstone-grpc/examples/golang/proto"
	"github.com/test-go/testify/assert"
)

func TestRetryQueue(t *testing.T) {
	pool := database.CreateTestDatabase(t, "test_solana_indexer_common")
	defer pool.Close()

	slot := uint64(123456)
	// Create a sample SubscribeUpdate
	update := &pb.SubscribeUpdate{
		UpdateOneof: &pb.SubscribeUpdate_Slot{
			Slot: &pb.SubscribeUpdateSlot{
				Slot: slot,
			},
		},
	}

	indexer := "test_indexer"
	errorMsg := "test error"

	// Add to retry queue
	err := AddToRetryQueue(t.Context(), pool, indexer, update, errorMsg)
	assert.NoError(t, err, "AddToRetryQueue should not error")

	// Get from retry queue
	items, err := GetRetryQueue(t.Context(), pool, 10, 0)
	assert.NoError(t, err, "GetRetryQueue should not error")
	assert.NotEmpty(t, items, "Retry queue should have at least one item")

	item := items[0]
	assert.Equal(t, indexer, item.Indexer)
	assert.Equal(t, errorMsg, item.Error)
	assert.NotNil(t, item.Update.SubscribeUpdate)
	assert.Equal(t, slot, item.Update.SubscribeUpdate.GetSlot().Slot)
	assert.NotNil(t, item.CreatedAt)
	assert.NotNil(t, item.UpdatedAt)

	// Delete from retry queue
	err = DeleteFromRetryQueue(t.Context(), pool, item.ID)
	assert.NoError(t, err, "DeleteFromRetryQueue should not error")

	// Ensure item is deleted
	items, err = GetRetryQueue(t.Context(), pool, 10, 0)
	assert.NoError(t, err, "GetRetryQueue after delete should not error")
	assert.Empty(t, items, "Retry queue should be empty after delete")
}
