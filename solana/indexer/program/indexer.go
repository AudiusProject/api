package program

import (
	"context"
	"fmt"
	"time"

	"api.audius.co/config"
	"api.audius.co/database"
	"api.audius.co/solana/indexer/common"
	"api.audius.co/solana/spl/programs/claimable_tokens"
	"api.audius.co/solana/spl/programs/payment_router"
	"api.audius.co/solana/spl/programs/reward_manager"
	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
	"github.com/maypok86/otter"
	pb "github.com/rpcpool/yellowstone-grpc/examples/golang/proto"
	"go.uber.org/zap"
)

const NAME = "program"

type Indexer struct {
	pool             database.DbPool
	grpcConfig       common.GrpcConfig
	rpcClient        common.RpcClient
	config           config.Config
	transactionCache *otter.Cache[solana.Signature, *rpc.GetTransactionResult]
	logger           *zap.Logger
}

func New(
	grpcConfig common.GrpcConfig,
	rpcClient common.RpcClient,
	pool database.DbPool,
	config config.Config,
	transactionCache *otter.Cache[solana.Signature, *rpc.GetTransactionResult],
	logger *zap.Logger,
) *Indexer {
	return &Indexer{
		pool:             pool,
		grpcConfig:       grpcConfig,
		rpcClient:        rpcClient,
		config:           config,
		transactionCache: transactionCache,
		logger:           logger.Named("ProgramIndexer"),
	}
}

func (d *Indexer) Start(ctx context.Context) {
	client, err := d.subscribe(ctx)
	if err != nil {
		d.logger.Fatal("failed to start subscription", zap.Error(err))
	}
	defer client.Close()

	// Wait for shutdown
	for {
		select {
		case <-ctx.Done():
			d.logger.Info("received shutdown signal, stopping indexer")
			return
		default:
		}
	}
}

func (d *Indexer) HandleUpdate(ctx context.Context, msg *pb.SubscribeUpdate) error {
	// Handle slot updates
	slotUpdate := msg.GetSlot()
	if slotUpdate != nil {
		// only update every 10 slots to reduce db load and write latency
		if slotUpdate.Slot%10 == 0 {
			// Use the filter as the checkpoint ID
			checkpointId := msg.Filters[0]

			err := common.UpdateCheckpoint(ctx, d.pool, checkpointId, slotUpdate.Slot)
			if err != nil {
				d.logger.Error("failed to update slot checkpoint", zap.Error(err))
			}
		}
	}

	// Handle transaction updates
	txUpdate := msg.GetTransaction()
	if txUpdate != nil {
		txSig := solana.SignatureFromBytes(txUpdate.Transaction.Signature)

		// Fetch the transaction details
		// Note: Could also convert the subscription transaction to a solana.Transaction,
		// but that could be error prone and the transaction is probably already in the cache anyway.
		// Also, we need the blocktime which the subscription doesn't seem to provide.
		txRes, err := common.FetchTransactionWithCache(ctx, d.transactionCache, d.rpcClient, txSig)
		if err != nil {
			return fmt.Errorf("failed to fetch transaction: %w", err)
		}

		// Decode the transaction
		tx, err := txRes.Transaction.GetTransaction()
		if err != nil {
			return fmt.Errorf("failed to decode transaction: %w", err)
		}

		// Add the lookup table accounts to the message accounts
		tx = common.ResolveLookupTables(ctx, d.rpcClient, tx, txRes.Meta)

		// Process the transaction
		d.processTransaction(ctx, txUpdate.Slot, txRes.Meta, tx, txRes.BlockTime.Time())

		return nil
	}

	return nil
}

func (d *Indexer) subscribe(ctx context.Context) (common.GrpcClient, error) {
	programIds := []string{
		d.config.SolanaConfig.RewardManagerProgramID.String(),
		d.config.SolanaConfig.PaymentRouterProgramID.String(),
		d.config.SolanaConfig.ClaimableTokensProgramID.String(),
	}

	d.logger.Info("subscribing to programs...", zap.Int("count", len(programIds)))
	subscription := d.makeSubscriptionRequest(ctx, programIds)
	handleMessage := func(ctx context.Context, update *pb.SubscribeUpdate) {
		err := d.HandleUpdate(ctx, update)
		if err != nil {
			d.logger.Error("failed to handle update", zap.Error(err))
			// Add messages that failed to process to the retry queue
			if err := common.AddToRetryQueue(ctx, d.pool, NAME, update, err.Error()); err != nil {
				d.logger.Error("failed to add to retry queue", zap.Error(err))
			}
		}
	}

	client := common.NewGrpcClient(d.grpcConfig)
	err := client.Subscribe(ctx, subscription, handleMessage, func(err error) {
		d.logger.Error("subscription error", zap.Error(err))
	})
	if err != nil {
		return nil, fmt.Errorf("failed to start subscription: %w", err)
	}

	d.logger.Info("subscribed to programs", zap.Int("count", len(programIds)))
	return client, nil
}

// Makes a subscription to the relevant program IDs and adds slot checkpointing
func (d *Indexer) makeSubscriptionRequest(ctx context.Context, programIds []string) *pb.SubscribeRequest {
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
	checkpointId, fromSlot, err := common.EnsureCheckpoint(ctx, NAME, d.pool, d.rpcClient, subscription, d.logger)
	if err != nil {
		d.logger.Error("failed to ensure checkpoint", zap.Error(err))
	}

	// Set the from slot for the subscription
	subscription.FromSlot = &fromSlot

	// Listen for slots for making checkpoints
	subscription.Slots = make(map[string]*pb.SubscribeRequestFilterSlots)
	subscription.Slots[checkpointId] = &pb.SubscribeRequestFilterSlots{}

	return subscription
}

func (d *Indexer) processTransaction(ctx context.Context, slot uint64, meta *rpc.TransactionMeta, tx *solana.Transaction, blockTime time.Time) error {
	signature := tx.Signatures[0].String()
	logger := d.logger.With(
		zap.String("signature", signature),
		zap.Uint64("slot", slot),
	)

	// Process balance changes for USDC (other tokens will be tracked by TokenIndexer)
	common.ProcessBalanceChanges(ctx, d.pool, slot, meta, tx, blockTime, []string{d.config.SolanaConfig.MintUSDC.String()}, logger)

	// Process individual instructions
	for instructionIndex, instruction := range tx.Message.Instructions {
		programId := tx.Message.AccountKeys[instruction.ProgramIDIndex]
		instLogger := logger.With(
			zap.String("programId", programId.String()),
			zap.Int("instructionIndex", instructionIndex),
		)
		switch programId {
		case claimable_tokens.ProgramID:
			{
				err := processClaimableTokensInstruction(ctx, d.pool, slot, tx, instructionIndex, instruction, signature, instLogger)
				if err != nil {
					return fmt.Errorf("error processing claimable_tokens instruction %d: %w", instructionIndex, err)
				}
			}
		case reward_manager.ProgramID:
			{
				err := processRewardManagerInstruction(ctx, d.pool, slot, tx, instructionIndex, instruction, signature, instLogger)
				if err != nil {
					return fmt.Errorf("error processing reward_manager instruction %d: %w", instructionIndex, err)
				}
			}
		case payment_router.ProgramID:
			{
				err := processPaymentRouterInstruction(ctx, d.pool, slot, tx, instructionIndex, instruction, signature, blockTime, d.config, instLogger)
				if err != nil {
					return fmt.Errorf("error processing payment_router instruction %d: %w", instructionIndex, err)
				}
			}
		}
	}

	return nil
}
