package dbc

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"api.audius.co/config"
	"api.audius.co/database"
	"api.audius.co/solana/indexer/common"
	"api.audius.co/solana/spl/programs/meteora_dbc"
	bin "github.com/gagliardetto/binary"
	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgxlisten"
	"github.com/maypok86/otter"
	pb "github.com/rpcpool/yellowstone-grpc/examples/golang/proto"
	"go.uber.org/zap"
)

const (
	NAME                       = "DbcIndexer"
	MAX_COINS_PER_SUBSCRIPTION = 10000 // Arbitrary
	NOTIFICATION_NAME          = "artist_coins_mint_changed"
)

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
		logger:           logger.Named(NAME),
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
	handleNotif := func(ctx context.Context, notification *pgconn.Notification, conn *pgx.Conn) error {
		subCtx, cancel := context.WithCancel(ctx)

		type notificationPayload struct {
			New string
			Old string
		}
		var n notificationPayload
		err := json.Unmarshal([]byte(notification.Payload), &n)
		if err != nil {
			d.logger.Error("failed to unmarshal notification payload", zap.String("payload", notification.Payload), zap.Error(err))
			// Proceed with resubscription even if unmarshalling fails
		} else {
			d.logger.Info("resubscribing due to dbc_pool change",
				zap.String("notification", notification.Channel),
				zap.String("new", n.New),
				zap.String("old", n.Old),
			)
		}

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
			cancel()
			return fmt.Errorf("failed to resubscribe to DBC pools: %w", err)
		}

		lastCancel = cancel
		return nil
	}

	// Setup initial subscription
	clients, err := d.subscribe(ctx)
	if err != nil {
		d.logger.Error("failed to subscribe to DBC pools", zap.Error(err))
		return
	}
	grpcClients = clients

	// Acquire the connection to be used by pgxlisten
	conn, err := d.pool.Acquire(ctx)
	if err != nil {
		d.logger.Error("failed to acquire database connection", zap.Error(err))
		return
	}
	defer conn.Release()

	// Setup a listener for pg_notify notifications
	listener := pgxlisten.Listener{
		Connect: func(ctx context.Context) (*pgx.Conn, error) {
			return conn.Conn(), nil
		},
		LogError: func(ctx context.Context, err error) {
			if !errors.Is(err, context.Canceled) {
				d.logger.Error("error occured in pg_notify subscription", zap.Error(err))
			}
		},
		ReconnectDelay: 1 * time.Second,
	}
	listener.Handle(NOTIFICATION_NAME, pgxlisten.HandlerFunc(handleNotif))

	// Start listening for notifications
	// this will block until the context is cancelled
	err = listener.Listen(ctx)
	if err != nil && !errors.Is(err, context.Canceled) {
		d.logger.Error("failed to start pgxlisten listener", zap.Error(err))
	}

	d.logger.Info("shutting down")
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
		// Handle DBC Config updates
		if len(accountUpdate.Account.Data) > 8 && bytes.Equal(accountUpdate.Account.Data[:8], meteora_dbc.POOL_CONFIG_DISCRIMINATOR) {
			var config meteora_dbc.PoolConfig
			err := bin.NewBorshDecoder(accountUpdate.Account.Data).Decode(&config)
			if err != nil {
				return fmt.Errorf("failed to decode DBC config account: %w", err)
			}
			account := solana.PublicKeyFromBytes(accountUpdate.Account.Pubkey)

			err = processDbcConfigUpdate(ctx, d.pool, accountUpdate.Slot, account, &config)
			if err != nil {
				return fmt.Errorf("failed to process DBC config update: %w", err)
			}
			d.logger.Debug("processed DBC config update",
				zap.String("account", account.String()),
			)
		}

		// Handle DBC Pool updates
		if len(accountUpdate.Account.Data) > 8 && bytes.Equal(accountUpdate.Account.Data[:8], meteora_dbc.POOL_DISCRIMINATOR) {
			var pool meteora_dbc.Pool
			err := bin.NewBorshDecoder(accountUpdate.Account.Data).Decode(&pool)
			if err != nil {
				return fmt.Errorf("failed to decode DBC pool account: %w", err)
			}

			account := solana.PublicKeyFromBytes(accountUpdate.Account.Pubkey)

			err = processDbcPoolUpdate(ctx, d.pool, accountUpdate.Slot, account, &pool)
			if err != nil {
				return fmt.Errorf("failed to process DBC pool update: %w", err)
			}
			d.logger.Debug("processed DBC pool update",
				zap.String("account", account.String()),
				zap.String("mint", pool.BaseMint.String()),
			)

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

				// Add the lookup table accounts to the message accounts
				tx = common.ResolveLookupTables(ctx, d.rpcClient, tx, txRes.Meta)

				// Process the transaction
				err = d.processTransaction(ctx, txRes.Slot, tx)
			}
		}
	}
	return nil
}

func (d *Indexer) subscribe(ctx context.Context) ([]common.GrpcClient, error) {
	done := false
	page := 0
	pageSize := MAX_COINS_PER_SUBSCRIPTION
	total := 0
	grpcClients := make([]common.GrpcClient, 0)
	for !done {
		mints, err := common.GetArtistCoinMints(ctx, d.pool, pageSize, page*pageSize)
		if err != nil {
			return nil, fmt.Errorf("failed to get pools: %w", err)
		}
		if len(mints) == 0 {
			d.logger.Info("no pool to subscribe to")
			return grpcClients, nil
		}
		total += len(mints)

		d.logger.Debug("subscribing to pools....", zap.Int("count", len(mints)))
		subscription := d.makeSubscriptionRequest(ctx, mints)

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

		var grpcClient common.GrpcClient
		if d.grpcConfig.UseFumarole {
			grpcClient = common.NewFumaroleAdapter(
				d.grpcConfig,
				fmt.Sprintf("audius-indexer-dbc-page-%d", page),
			)
		} else {
			grpcClient = common.NewGrpcClient(d.grpcConfig)
		}
		err = grpcClient.Subscribe(ctx, subscription, handleMessage, func(err error) {
			d.logger.Error("error in subscription", zap.Error(err))
		})
		if err != nil {
			return nil, fmt.Errorf("failed to subscribe to pools: %w", err)
		}
		grpcClients = append(grpcClients, grpcClient)

		if len(mints) < pageSize {
			done = true
		}
		page++
	}
	d.logger.Info("subscribed to pools", zap.Int("count", total))
	return grpcClients, nil
}

func (d *Indexer) makeSubscriptionRequest(ctx context.Context, mints []string) *pb.SubscribeRequest {
	commitment := pb.CommitmentLevel_CONFIRMED
	subscription := &pb.SubscribeRequest{
		Commitment: &commitment,
	}

	// Listen to all watched pools
	subscription.Accounts = make(map[string]*pb.SubscribeRequestFilterAccounts)
	for _, mint := range mints {
		poolFilter := pb.SubscribeRequestFilterAccounts{
			Owner: []string{meteora_dbc.ProgramID.String()},
			Filters: []*pb.SubscribeRequestFilterAccountsFilter{
				{
					Filter: &pb.SubscribeRequestFilterAccountsFilter_Memcmp{
						Memcmp: &pb.SubscribeRequestFilterAccountsFilterMemcmp{
							Offset: 0,
							Data: &pb.SubscribeRequestFilterAccountsFilterMemcmp_Bytes{
								Bytes: meteora_dbc.POOL_DISCRIMINATOR,
							},
						},
					},
				},
				{
					Filter: &pb.SubscribeRequestFilterAccountsFilter_Memcmp{
						Memcmp: &pb.SubscribeRequestFilterAccountsFilterMemcmp{
							// Base mint is after discriminator, VolatilityTracker, config and creator
							Offset: 8 + (8 + 8 + 16 + 16 + 16) + 32 + 32,
							Data: &pb.SubscribeRequestFilterAccountsFilterMemcmp_Base58{
								Base58: mint,
							},
						},
					},
				},
			},
		}
		subscription.Accounts[mint] = &poolFilter
	}

	configFilter := pb.SubscribeRequestFilterAccounts{
		Owner: []string{meteora_dbc.ProgramID.String()},
		Filters: []*pb.SubscribeRequestFilterAccountsFilter{
			{
				Filter: &pb.SubscribeRequestFilterAccountsFilter_Memcmp{
					Memcmp: &pb.SubscribeRequestFilterAccountsFilterMemcmp{
						Offset: 0,
						Data: &pb.SubscribeRequestFilterAccountsFilterMemcmp_Bytes{
							Bytes: meteora_dbc.POOL_CONFIG_DISCRIMINATOR,
						},
					},
				},
			},
			{
				Filter: &pb.SubscribeRequestFilterAccountsFilter_Memcmp{
					Memcmp: &pb.SubscribeRequestFilterAccountsFilterMemcmp{
						// Quote mint is after discriminator
						Offset: 8,
						Data: &pb.SubscribeRequestFilterAccountsFilterMemcmp_Base58{
							Base58: d.config.SolanaConfig.MintAudio.String(),
						},
					},
				},
			},
		},
	}
	subscription.Accounts["dbc_config"] = &configFilter

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

func processDbcPoolUpdate(
	ctx context.Context,
	db database.DbPool,
	slot uint64,
	account solana.PublicKey,
	pool *meteora_dbc.Pool,
) error {
	sqlTx, err := db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer sqlTx.Rollback(ctx)

	err = upsertDbcPool(ctx, sqlTx, slot, account, pool)
	if err != nil {
		return fmt.Errorf("failed to upsert DBC pool: %w", err)
	}
	err = upsertDbcPoolMetrics(ctx, sqlTx, slot, account, &pool.Metrics)
	if err != nil {
		return fmt.Errorf("failed to upsert DBC pool metrics: %w", err)
	}
	err = upsertDbcPoolVolatilityTracker(ctx, sqlTx, slot, account, &pool.VolatilityTracker)
	if err != nil {
		return fmt.Errorf("failed to upsert DBC volatility tracker: %w", err)
	}
	err = sqlTx.Commit(ctx)
	if err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}
	return nil
}

func processDbcConfigUpdate(
	ctx context.Context,
	db database.DbPool,
	slot uint64,
	account solana.PublicKey,
	config *meteora_dbc.PoolConfig,
) error {
	sqlTx, err := db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer sqlTx.Rollback(ctx)

	err = upsertDbcConfig(ctx, sqlTx, slot, account.String(), config)
	if err != nil {
		return fmt.Errorf("failed to upsert DBC pool config: %w", err)
	}
	err = upsertDbcConfigFees(ctx, sqlTx, slot, account.String(), &config.PoolFees)
	if err != nil {
		return fmt.Errorf("failed to upsert DBC config fees: %w", err)
	}
	err = upsertDbcConfigVesting(ctx, sqlTx, slot, account.String(), &config.LockedVestingConfig)
	if err != nil {
		return fmt.Errorf("failed to upsert DBC config vestings: %w", err)
	}
	err = sqlTx.Commit(ctx)
	if err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}
	return nil
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
