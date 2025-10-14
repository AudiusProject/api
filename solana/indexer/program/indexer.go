package program

import (
	"context"
	"encoding/json"
	"fmt"

	"api.audius.co/config"
	"api.audius.co/database"
	"api.audius.co/solana/indexer/common"
	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
	pb "github.com/rpcpool/yellowstone-grpc/examples/golang/proto"
	"go.uber.org/zap"
)

const NAME = "program"

type Indexer struct {
	pool       database.DbPool
	grpcConfig common.GrpcConfig
	rpcClient  common.RpcClient
	config     config.SolanaConfig
	logger     *zap.Logger
}

func New(
	grpcConfig common.GrpcConfig,
	rpcClient common.RpcClient,
	pool database.DbPool,
	config config.SolanaConfig,
	logger *zap.Logger,
) *Indexer {
	return &Indexer{
		pool:       pool,
		grpcConfig: grpcConfig,
		rpcClient:  rpcClient,
		config:     config,
		logger:     logger.Named("ProgramIndexer"),
	}
}

func (i *Indexer) Start(ctx context.Context) {
	client, err := i.subscribe(ctx)
	if err != nil {
		i.logger.Fatal("failed to start subscription", zap.Error(err))
	}
	defer client.Close()

	i.logger.Info("subscribed")

	// Wait for shutdown
	for {
		select {
		case <-ctx.Done():
			i.logger.Info("received shutdown signal, stopping indexer")
			return
		default:
		}
	}
}

func (i *Indexer) subscribe(ctx context.Context) (common.GrpcClient, error) {
	programIds := []string{
		i.config.RewardManagerProgramID.String(),
		i.config.PaymentRouterProgramID.String(),
		i.config.ClaimableTokensProgramID.String(),
	}

	subscription := i.makeSubscriptionRequest(ctx, programIds)

	handleMessage := func(ctx context.Context, update *pb.SubscribeUpdate) {
		err := i.HandleUpdate(ctx, update)
		if err != nil {
			i.logger.Error("failed to handle update", zap.Error(err))

			// Add messages that failed to process to the retry queue
			if err := common.AddToRetryQueue(ctx, i.pool, NAME, update, err.Error()); err != nil {
				i.logger.Error("failed to add to retry queue", zap.Error(err))
			}
		}
	}

	client := common.NewGrpcClient(i.grpcConfig)
	err := client.Subscribe(ctx, subscription, handleMessage, func(err error) {
		i.logger.Error("subscription error", zap.Error(err))
	})
	if err != nil {
		return nil, fmt.Errorf("failed to start subscription: %w", err)
	}
	return client, nil
}

func (i *Indexer) makeSubscriptionRequest(ctx context.Context, programIds []string) *pb.SubscribeRequest {
	commitment := pb.CommitmentLevel_CONFIRMED
	subscription := &pb.SubscribeRequest{
		Commitment: &commitment,
	}

	// Filter to only the relevant program IDs
	subscription.Transactions = make(map[string]*pb.SubscribeRequestFilterTransactions)
	subscription.Transactions[NAME] = &pb.SubscribeRequestFilterTransactions{
		AccountInclude: programIds,
	}

	// Ensure this subscription has a checkpoint
	checkpointId, fromSlot, err := common.EnsureCheckpoint(ctx, NAME, i.pool, i.rpcClient, subscription, i.logger)
	if err != nil {
		i.logger.Error("failed to ensure checkpoint", zap.Error(err))
	}

	// Set the from slot for the subscription
	subscription.FromSlot = &fromSlot

	// Listen for slots for making checkpoints
	subscription.Slots = make(map[string]*pb.SubscribeRequestFilterSlots)
	subscription.Slots[checkpointId] = &pb.SubscribeRequestFilterSlots{}

	return subscription
}

func (i *Indexer) HandleUpdate(ctx context.Context, msg *pb.SubscribeUpdate) error {
	// Handle slot updates
	slotUpdate := msg.GetSlot()
	if slotUpdate != nil {
		// only update every 10 slots to reduce db load and write latency
		if slotUpdate.Slot%10 == 0 {
			// Use the filter as the checkpoint ID
			checkpointId := msg.Filters[0]

			err := common.UpdateCheckpoint(ctx, i.pool, checkpointId, slotUpdate.Slot)
			if err != nil {
				i.logger.Error("failed to update slot checkpoint", zap.Error(err))
			}
		}
	}

	// Handle transaction updates
	txUpdate := msg.GetTransaction()
	if txUpdate != nil {
		i.logger.Debug("processing transaction...", zap.String("signature", string(txUpdate.Transaction.Transaction.Signatures[0])), zap.Uint64("slot", txUpdate.Slot))

		bytes, err := json.Marshal(txUpdate.Transaction.Transaction)
		if err != nil {
			return fmt.Errorf("failed to marshal transaction: %w", err)
		}

		var tx solana.Transaction
		err = json.Unmarshal(bytes, &tx)
		if err != nil {
			return fmt.Errorf("failed to unmarshal transaction: %w", err)
		}

		metaJson, err := json.Marshal(txUpdate.Transaction.Meta)
		if err != nil {
			return fmt.Errorf("failed to marshal transaction meta: %w", err)
		}

		var meta rpc.TransactionMeta
		err = json.Unmarshal(metaJson, &meta)
		if err != nil {
			return fmt.Errorf("failed to unmarshal transaction meta: %w", err)
		}

		tx = *common.ResolveLookupTables(ctx, i.rpcClient, &tx, &meta)

	}

	return nil
}
