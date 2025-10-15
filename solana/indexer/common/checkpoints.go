package common

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"api.audius.co/database"
	"github.com/jackc/pgx/v5"
	pb "github.com/rpcpool/yellowstone-grpc/examples/golang/proto"
	"go.uber.org/zap"
)

const MAX_SLOT_GAP = 2500

func EnsureCheckpoint(
	ctx context.Context,
	name string,
	db database.DBTX,
	rpcClient RpcClient,
	subscription *pb.SubscribeRequest,
	logger *zap.Logger,
) (string, uint64, error) {
	lastIndexedSlot, err := GetCheckpointSlot(ctx, db, name, subscription)
	if err != nil {
		return "", 0, fmt.Errorf("failed to get last indexed slot: %w", err)
	}

	latestSlot, err := WithRetriesResult(func() (uint64, error) {
		return rpcClient.GetSlot(ctx, "confirmed")
	}, 5, time.Second*2)
	if err != nil {
		return "", 0, fmt.Errorf("failed to get slot: %w", err)
	}

	var fromSlot uint64
	minimumSlot := uint64(0)
	if latestSlot > MAX_SLOT_GAP {
		minimumSlot = latestSlot - MAX_SLOT_GAP
	}

	if lastIndexedSlot > minimumSlot {
		// Existing subscription reconnecting, continue from last indexed slot
		fromSlot = lastIndexedSlot
	} else if lastIndexedSlot == 0 {
		// New subscription, continue from latest slot - 100
		fromSlot = latestSlot - 100 // start 100 slots back to be safe
		logger.Warn("no last indexed slot found, starting from most recent slot (less 100 for safety) and skipping backfill", zap.Uint64("fromSlot", fromSlot))
	} else {
		// Existing subscription that's too old, continue from as far back as possible
		fromSlot = minimumSlot
		logger.Warn("last indexed slot is too old, starting from minimum slot", zap.Uint64("fromSlot", fromSlot), zap.Uint64("toSlot", lastIndexedSlot))
	}

	checkpointId, err := InsertCheckpointStart(ctx, db, name, fromSlot, subscription)
	if err != nil {
		return "", 0, fmt.Errorf("failed to start checkpoint: %w", err)
	}

	return checkpointId, fromSlot, nil
}

func InsertBackfillCheckpoint(ctx context.Context, db database.DBTX, fromSlot uint64, toSlot uint64, address string) (string, error) {
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

func InsertCheckpointStart(
	ctx context.Context,
	db database.DBTX,
	name string,
	fromSlot uint64,
	subscription *pb.SubscribeRequest,
) (string, error) {
	subscriptionJson, err := json.Marshal(subscription)
	if err != nil {
		return "", fmt.Errorf("failed to marshal subscription request: %w", err)
	}

	sum := sha256.Sum256(subscriptionJson)
	subscriptionHash := hex.EncodeToString(sum[:])

	var checkpointId string
	err = db.QueryRow(ctx, `
		INSERT INTO sol_slot_checkpoints (name, from_slot, to_slot, subscription, subscription_hash)
		VALUES (@name, @from_slot, @to_slot, @subscription, @subscription_hash)
		RETURNING id;
	`, pgx.NamedArgs{
		"name":              name,
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

func UpdateCheckpoint(ctx context.Context, db database.DBTX, id string, slot uint64) error {
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

func GetCheckpointSlot(ctx context.Context, db database.DBTX, name string, subscription *pb.SubscribeRequest) (uint64, error) {
	subscriptionJson, err := json.Marshal(subscription)
	if err != nil {
		return 0, fmt.Errorf("failed to marshal subscription request: %w", err)
	}

	sum := sha256.Sum256(subscriptionJson)
	subscriptionHash := hex.EncodeToString(sum[:])

	sql := `
		SELECT COALESCE(MAX(to_slot), 0)
		FROM sol_slot_checkpoints
		WHERE name = @name
			AND subscription_hash = @subscription_hash
		LIMIT 1;
	`

	var lastIndexedSlot uint64
	err = db.QueryRow(ctx, sql, pgx.NamedArgs{"name": name, "subscription_hash": subscriptionHash}).Scan(&lastIndexedSlot)
	if err != nil && err != pgx.ErrNoRows {
		return 0, fmt.Errorf("failed to scan last slot: %w", err)
	}
	return lastIndexedSlot, nil
}
