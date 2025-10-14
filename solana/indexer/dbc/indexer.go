package dbc

import (
	"context"
	"fmt"

	"api.audius.co/database"
	"api.audius.co/solana/indexer/common"
	"api.audius.co/solana/spl/programs/meteora_damm_v2"
	"api.audius.co/solana/spl/programs/meteora_dbc"
	bin "github.com/gagliardetto/binary"
	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/maypok86/otter"
	pb "github.com/rpcpool/yellowstone-grpc/examples/golang/proto"
	"go.uber.org/zap"
)

const (
	NAME                       = "dbc"
	MAX_POOLS_PER_SUBSCRIPTION = 10000 // Arbitrary
	NOTIFICATION_NAME          = "artist_coins_dbc_pool_changed"
)

type Indexer struct {
	pool             database.DbPool
	grpcConfig       common.GrpcConfig
	rpcClient        common.RpcClient
	transactionCache *otter.Cache[solana.Signature, *rpc.GetTransactionResult]
	logger           *zap.Logger
}

func New(
	grpcConfig common.GrpcConfig,
	rpcClient common.RpcClient,
	pool database.DbPool,
	transactionCache *otter.Cache[solana.Signature, *rpc.GetTransactionResult],
	logger *zap.Logger,
) *Indexer {
	return &Indexer{
		pool:             pool,
		grpcConfig:       grpcConfig,
		rpcClient:        rpcClient,
		transactionCache: transactionCache,
		logger:           logger.Named("DBCIndexer"),
	}
}

func (d *Indexer) Start(ctx context.Context) {
	// To ensure only one subscription task is running at a time, keep track of
	// the last cancel function and call it on the next notification.
	var lastCancel context.CancelFunc

	// Ensure all gRPC clients are closed on shutdown
	var grpcClients []common.GrpcClient
	defer (func() {
		for _, client := range grpcClients {
			client.Close()
		}
	})()

	// On notification, cancel the previous subscription task (if any) and start a new one
	handleNotif := func(ctx context.Context, notification *pgconn.Notification) {
		subCtx, cancel := context.WithCancel(ctx)

		// Cancel previous subscription task
		if lastCancel != nil {
			lastCancel()
		}

		// Close previous gRPC clients
		for _, client := range grpcClients {
			client.Close()
		}

		// Resubscribe to all DBC pools
		// TODO: Optimize this to only add/remove DBC pools instead of resubscribing to all
		clients, err := d.subscribe(subCtx)
		grpcClients = clients
		if err != nil {
			d.logger.Error("failed to resubscribe to DBC pools", zap.Error(err))
			cancel()
			return
		}

		lastCancel = cancel
	}

	// Setup initial subscription
	clients, err := d.subscribe(ctx)
	if err != nil {
		d.logger.Error("failed to subscribe to DAMM V2 pools", zap.Error(err))
		return
	}
	grpcClients = clients

	// Watch for new pools to be added
	err = common.WatchPgNotification(ctx, d.pool, NOTIFICATION_NAME, handleNotif, d.logger)
	if err != nil {
		d.logger.Error("failed to watch for DBC pool changes", zap.Error(err))
		return
	}

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

	// Handle account updates
	accountUpdate := msg.GetAccount()
	if accountUpdate != nil {
		var pool meteora_dbc.Pool
		err := bin.NewBorshDecoder(accountUpdate.Account.Data).Decode(&pool)
		if err != nil {
			return fmt.Errorf("failed to decode DBC pool account: %w", err)
		}

		account := solana.PublicKeyFromBytes(accountUpdate.Account.Pubkey)
		err = upsertDbcPool(ctx, d.pool, accountUpdate.Slot, account, &pool)
		if err != nil {
			return fmt.Errorf("failed to upsert DBC pool: %w", err)
		}

		// If the pool is migrated, check for the migration transaction and process it
		if pool.IsMigrated == uint8(1) {
			txSig := solana.SignatureFromBytes(accountUpdate.Account.TxnSignature)

			// Fetch the transaction details
			txRes, err := common.FetchTransactionWithCache(ctx, d.transactionCache, d.rpcClient, txSig)
			if err != nil {
				return fmt.Errorf("failed to fetch transaction: %w", err)
			}

			// Decode the transaction
			tx, err := txRes.Transaction.GetTransaction()
			if err != nil {
				return fmt.Errorf("failed to decode transaction: %w", err)
			}

			// Process the transaction
			err = d.processTransaction(ctx, accountUpdate.Slot, tx)
		}
	}
	return nil
}

func (d *Indexer) subscribe(ctx context.Context) ([]common.GrpcClient, error) {
	done := false
	page := 0
	pageSize := MAX_POOLS_PER_SUBSCRIPTION
	total := 0
	grpcClients := make([]common.GrpcClient, 0)
	for !done {
		pools, err := getSubscribedDbcPools(ctx, d.pool, pageSize, page*pageSize)
		if err != nil {
			return nil, fmt.Errorf("failed to get pools: %w", err)
		}
		if len(pools) == 0 {
			d.logger.Info("no pools to subscribe to")
			return grpcClients, nil
		}
		total += len(pools)

		d.logger.Debug("subscribing to pools....", zap.Int("numPools", len(pools)))
		subscription := d.makeSubscriptionRequest(ctx, pools)

		// Handle each message from the subscription
		handleMessage := func(ctx context.Context, msg *pb.SubscribeUpdate) {
			err := d.HandleUpdate(ctx, msg)
			if err != nil {
				d.logger.Error("failed to handle update", zap.Error(err))

				// Add messages that failed to process to the retry queue
				if err := common.AddToRetryQueue(ctx, d.pool, NAME, msg, err.Error()); err != nil {
					d.logger.Error("failed to add to retry queue", zap.Error(err))
				}
			}
		}

		grpcClient := common.NewGrpcClient(d.grpcConfig)
		err = grpcClient.Subscribe(ctx, subscription, handleMessage, func(err error) {
			d.logger.Error("error in subscription", zap.Error(err))
		})
		if err != nil {
			return nil, fmt.Errorf("failed to subscribe to pools: %w", err)
		}
		grpcClients = append(grpcClients, grpcClient)

		if len(pools) < pageSize {
			done = true
		}
		page++
	}
	d.logger.Info("subscribed to pools", zap.Int("count", total))
	return grpcClients, nil
}

func (d *Indexer) makeSubscriptionRequest(ctx context.Context, pools []string) *pb.SubscribeRequest {
	commitment := pb.CommitmentLevel_CONFIRMED
	subscription := &pb.SubscribeRequest{
		Commitment: &commitment,
	}

	// Listen to all watched pools
	subscription.Accounts = make(map[string]*pb.SubscribeRequestFilterAccounts)
	accountFilter := pb.SubscribeRequestFilterAccounts{
		Owner:   []string{meteora_damm_v2.ProgramID.String()},
		Account: pools,
	}
	subscription.Accounts[NAME] = &accountFilter

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

func (i *Indexer) processTransaction(ctx context.Context, slot uint64, tx *solana.Transaction) error {
	signature := tx.Signatures[0].String()
	logger := i.logger.With(
		zap.String("signature", signature),
		zap.Uint64("slot", slot),
	)

	// Process individual instructions
	for instructionIndex, instruction := range tx.Message.Instructions {
		programId := tx.Message.AccountKeys[instruction.ProgramIDIndex]
		instLogger := logger.With(
			zap.String("programId", programId.String()),
			zap.Int("instructionIndex", instructionIndex),
		)
		switch programId {
		case meteora_dbc.ProgramID:
			{
				err := processDbcInstruction(ctx, i.pool, slot, tx, instructionIndex, instruction, signature, instLogger)
				if err != nil {
					return fmt.Errorf("error processing meteora_dbc instruction %d: %w", instructionIndex, err)
				}
			}
		}
	}
	return nil
}

// Gets the canonical DBC pools from the artist coins table.
func getSubscribedDbcPools(ctx context.Context, db database.DBTX, limit int, offset int) ([]string, error) {
	sql := `
		SELECT dbc_pool
		FROM artist_coins
		WHERE dbc_pool IS NOT NULL
		LIMIT @limit OFFSET @offset
	;`
	rows, err := db.Query(ctx, sql, pgx.NamedArgs{
		"limit":  limit,
		"offset": offset,
	})
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var pools []string
	for rows.Next() {
		var address string
		if err := rows.Scan(&address); err != nil {
			return nil, err
		}
		pools = append(pools, address)
	}
	return pools, nil
}

func upsertDbcPool(
	ctx context.Context,
	db database.DBTX,
	slot uint64,
	account solana.PublicKey,
	pool *meteora_dbc.Pool,
) error {
	sql := `
		INSERT INTO sol_meteora_dbc_pools (
			address,
			slot,
			config,
			creator,
			base_mint,
			base_vault,
			quote_vault,
			base_reserve,
			quote_reserve,
			protocol_base_fee,
			partner_base_fee,
			partner_quote_fee,
			sqrt_price,
			activation_point,
			pool_type,
			is_migrated,
			is_partner_withdraw_surplus,
			is_protocol_withdraw_surplus,
			migration_progress,
			is_withdraw_leftover,
			is_creator_withdraw_surplus,
			migration_fee_withdraw_status,
			finish_curve_timestamp,
			creator_base_fee,
			creator_quote_fee,
			created_at,
			updated_at
		) VALUES (
			@address,
			@slot,
			@config,
			@creator,
			@base_mint,
			@base_vault,
			@quote_vault,
			@base_reserve,
			@quote_reserve,
			@protocol_base_fee,
			@partner_base_fee,
			@partner_quote_fee,
			@sqrt_price,
			@activation_point,
			@pool_type,
			@is_migrated,
			@is_partner_withdraw_surplus,
			@is_protocol_withdraw_surplus,
			@migration_progress,
			@is_withdraw_leftover,
			@is_creator_withdraw_surplus,
			@migration_fee_withdraw_status,
			@finish_curve_timestamp,
			@creator_base_fee,
			@creator_quote_fee,
			NOW(),
			NOW()
		) ON CONFLICT (address) DO UPDATE SET
			slot = EXCLUDED.slot,
			config = EXCLUDED.config,
			creator = EXCLUDED.creator,
			base_mint = EXCLUDED.base_mint,
			base_vault = EXCLUDED.base_vault,
			quote_vault = EXCLUDED.quote_vault,
			base_reserve = EXCLUDED.base_reserve,
			quote_reserve = EXCLUDED.quote_reserve,
			protocol_base_fee = EXCLUDED.protocol_base_fee,
			partner_base_fee = EXCLUDED.partner_base_fee,
			partner_quote_fee = EXCLUDED.partner_quote_fee,
			sqrt_price = EXCLUDED.sqrt_price,
			activation_point = EXCLUDED.activation_point,
			pool_type = EXCLUDED.pool_type,
			is_migrated = EXCLUDED.is_migrated,
			is_partner_withdraw_surplus = EXCLUDED.is_partner_withdraw_surplus,
			is_protocol_withdraw_surplus = EXCLUDED.is_protocol_withdraw_surplus,
			migration_progress = EXCLUDED.migration_progress,
			is_withdraw_leftover = EXCLUDED.is_withdraw_leftover,
			is_creator_withdraw_surplus = EXCLUDED.is_creator_withdraw_surplus,
			migration_fee_withdraw_status = EXCLUDED.migration_fee_withdraw_status,
			finish_curve_timestamp = EXCLUDED.finish_curve_timestamp,
			creator_base_fee = EXCLUDED.creator_base_fee,
			creator_quote_fee = EXCLUDED.creator_quote_fee,
			updated_at = NOW()
	;`
	_, err := db.Exec(ctx, sql, pgx.NamedArgs{
		"address":                       account.String(),
		"slot":                          slot,
		"config":                        pool.Config.String(),
		"creator":                       pool.Creator.String(),
		"base_mint":                     pool.BaseMint.String(),
		"base_vault":                    pool.BaseVault.String(),
		"quote_vault":                   pool.QuoteVault.String(),
		"base_reserve":                  pool.BaseReserve,
		"quote_reserve":                 pool.QuoteReserve,
		"protocol_base_fee":             pool.ProtocolBaseFee,
		"partner_base_fee":              pool.PartnerBaseFee,
		"partner_quote_fee":             pool.PartnerQuoteFee,
		"sqrt_price":                    pool.SqrtPrice.BigInt(),
		"activation_point":              pool.ActivationPoint,
		"pool_type":                     pool.PoolType,
		"is_migrated":                   pool.IsMigrated,
		"is_partner_withdraw_surplus":   pool.IsPartnerWithdrawSurplus,
		"is_protocol_withdraw_surplus":  pool.IsProtocolWithdrawSurplus,
		"migration_progress":            pool.MigrationProgress,
		"is_withdraw_leftover":          pool.IsWithdrawLeftover,
		"is_creator_withdraw_surplus":   pool.IsCreatorWithdrawSurplus,
		"migration_fee_withdraw_status": pool.MigrationFeeWithdrawStatus,
		"finish_curve_timestamp":        pool.FinishCurveTimestamp,
		"creator_base_fee":              pool.CreatorBaseFee,
		"creator_quote_fee":             pool.CreatorQuoteFee,
	})
	return err
}
