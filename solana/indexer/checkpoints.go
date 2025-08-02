package indexer

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"

	"bridgerton.audius.co/database"
	"github.com/jackc/pgx/v5"
	pb "github.com/rpcpool/yellowstone-grpc/examples/golang/proto"
)

func insertBackfillCheckpoint(ctx context.Context, db database.DBTX, fromSlot uint64, toSlot uint64, address string) (string, error) {
	obj := map[string]string{
		"type":    "backfill",
		"address": address,
	}
	subscriptionJson, err := json.Marshal(obj)
	if err != nil {
		return "", fmt.Errorf("failed to marshal backfill subscription: %w", err)
	}

	sum := sha256.Sum256(subscriptionJson)
	subscriptionHash := hex.EncodeToString(sum[:])

	var checkpointId string
	err = db.QueryRow(ctx, `
			INSERT INTO sol_slot_checkpoints (from_slot, to_slot, subscription, subscription_hash) 
			VALUES (@from_slot, @to_slot, @subscription, @subscription_hash)
			RETURNING id;
		`, pgx.NamedArgs{
		"from_slot":         fromSlot,
		"to_slot":           toSlot,
		"subscription":      string(subscriptionJson),
		"subscription_hash": subscriptionHash,
	}).Scan(&checkpointId)

	if err != nil {
		return "", fmt.Errorf("failed to insert slot checkpoint: %w", err)
	}

	return checkpointId, nil
}

func insertCheckpointStart(ctx context.Context, db database.DBTX, fromSlot uint64, subscription *pb.SubscribeRequest) (string, error) {
	subscriptionJson, err := json.Marshal(subscription)
	if err != nil {
		return "", fmt.Errorf("failed to marshal subscription request: %w", err)
	}

	sum := sha256.Sum256(subscriptionJson)
	subscriptionHash := hex.EncodeToString(sum[:])

	var checkpointId string
	err = db.QueryRow(ctx, `
		INSERT INTO sol_slot_checkpoints (from_slot, to_slot, subscription, subscription_hash) 
		VALUES (@from_slot, @to_slot, @subscription, @subscription_hash)
		RETURNING id;
	`, pgx.NamedArgs{
		"from_slot":         fromSlot,
		"to_slot":           fromSlot,
		"subscription":      string(subscriptionJson),
		"subscription_hash": subscriptionHash,
	}).Scan(&checkpointId)

	if err != nil {
		return "", fmt.Errorf("failed to insert slot checkpoint: %w", err)
	}

	return checkpointId, nil
}

func updateCheckpoint(ctx context.Context, db database.DBTX, id string, slot uint64) error {
	_, err := db.Exec(ctx, `
			UPDATE sol_slot_checkpoints
			SET to_slot = @to_slot,
				updated_at = NOW()
			WHERE id = @id
				AND to_slot < @to_slot;
		`, pgx.NamedArgs{
		"to_slot": slot,
		"id":      id,
	})
	return err
}

func getCheckpointSlot(ctx context.Context, db database.DBTX, subscription *pb.SubscribeRequest) (uint64, error) {
	subscriptionJson, err := json.Marshal(subscription)
	if err != nil {
		return 0, fmt.Errorf("failed to marshal subscription request: %w", err)
	}

	sum := sha256.Sum256(subscriptionJson)
	subscriptionHash := hex.EncodeToString(sum[:])

	sql := `
		SELECT COALESCE(MAX(to_slot), 0) 
		FROM sol_slot_checkpoints 
		WHERE subscription_hash = @subscription_hash 
		LIMIT 1;
	`

	var lastIndexedSlot uint64
	err = db.QueryRow(ctx, sql, pgx.NamedArgs{"subscription_hash": subscriptionHash}).Scan(&lastIndexedSlot)
	if err != nil && err != pgx.ErrNoRows {
		return 0, fmt.Errorf("failed to scan last slot: %w", err)
	}
	return lastIndexedSlot, nil
}
