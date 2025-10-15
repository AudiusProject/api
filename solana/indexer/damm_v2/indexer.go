package damm_v2

import (
	"context"
	"fmt"

	"api.audius.co/database"
	"api.audius.co/solana/indexer/common"
	"api.audius.co/solana/spl/programs/meteora_damm_v2"
	bin "github.com/gagliardetto/binary"
	"github.com/gagliardetto/solana-go"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	pb "github.com/rpcpool/yellowstone-grpc/examples/golang/proto"
	"go.uber.org/zap"
)

const (
	NAME                       = "damm_v2"
	MAX_POOLS_PER_SUBSCRIPTION = 10000 // Arbitrary
	NOTIFICATION_NAME          = "artist_coins_damm_v2_pool_changed"
)

type Indexer struct {
	pool        database.DbPool
	grpcConfig  common.GrpcConfig
	grpcFactory func(common.GrpcConfig) common.GrpcClient
	rpcClient   common.RpcClient
	logger      *zap.Logger
}

func New(
	config common.GrpcConfig,
	rpcClient common.RpcClient,
	pool database.DbPool,
	logger *zap.Logger,
) *Indexer {
	return &Indexer{
		pool:        pool,
		grpcConfig:  config,
		grpcFactory: common.NewGrpcClient,
		rpcClient:   rpcClient,
		logger:      logger.Named("DammV2Indexer"),
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

		// Resubscribe to all DAMM V2 pools
		// TODO: Optimize this to only add/remove DAMM V2 pools instead of resubscribing to all
		clients, err := d.subscribe(subCtx)
		grpcClients = clients
		if err != nil {
			d.logger.Error("failed to resubscribe to DAMM V2 pools", zap.Error(err))
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
		d.logger.Error("failed to watch for DAMM V2 pool changes", zap.Error(err))
		return
	}

	// Wait for shutdown
	for {
		select {
		case <-ctx.Done():
			d.logger.Info("received shutdown signal, stopping DAMM V2 indexer")
			return
		default:
		}
	}
}

// Handles a single update message from the gRPC subscription
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

	// Handle DAMM V2 account updates
	accUpdate := msg.GetAccount()
	if accUpdate != nil {
		if msg.Filters[0] == NAME {
			err := processDammV2PoolUpdate(ctx, d.pool, accUpdate)
			if err != nil {
				return fmt.Errorf("failed to process DAMM V2 pool update: %w", err)
			}
			d.logger.Debug("processed DAMM V2 pool update", zap.String("account", solana.PublicKeyFromBytes(accUpdate.Account.Pubkey).String()))
		} else {
			err := processDammV2PositionUpdate(ctx, d.pool, accUpdate)
			if err != nil {
				return fmt.Errorf("failed to process DAMM V2 position update: %w", err)
			}
			d.logger.Debug("processed DAMM V2 position update", zap.String("account", solana.PublicKeyFromBytes(accUpdate.Account.Pubkey).String()))
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
		pools, err := getSubscribedDammV2Pools(ctx, d.pool, pageSize, page*pageSize)
		if err != nil {
			return nil, fmt.Errorf("failed to get pools: %w", err)
		}
		if len(pools) == 0 {
			d.logger.Info("no pools to subscribe to")
			return grpcClients, nil
		}
		total += len(pools)

		d.logger.Debug("subscribing to pools....", zap.Int("count", len(pools)))
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

		grpcClient := d.grpcFactory(d.grpcConfig)
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

func (d *Indexer) makeSubscriptionRequest(ctx context.Context, dammV2Pools []string) *pb.SubscribeRequest {
	commitment := pb.CommitmentLevel_CONFIRMED
	subscription := &pb.SubscribeRequest{
		Commitment: &commitment,
	}

	// Listen to all watched pools
	subscription.Accounts = make(map[string]*pb.SubscribeRequestFilterAccounts)
	accountFilter := pb.SubscribeRequestFilterAccounts{
		Owner:   []string{meteora_damm_v2.ProgramID.String()},
		Account: dammV2Pools,
	}
	subscription.Accounts[NAME] = &accountFilter

	// Listen to all positions for each pool
	for _, pool := range dammV2Pools {
		accountFilter := pb.SubscribeRequestFilterAccounts{
			Owner: []string{meteora_damm_v2.ProgramID.String()},
			Filters: []*pb.SubscribeRequestFilterAccountsFilter{
				{
					Filter: &pb.SubscribeRequestFilterAccountsFilter_Memcmp{
						Memcmp: &pb.SubscribeRequestFilterAccountsFilterMemcmp{
							Offset: 8, // Offset of the pool field in the position account (after discriminator)
							Data: &pb.SubscribeRequestFilterAccountsFilterMemcmp_Base58{
								Base58: pool,
							},
						},
					},
				},
				{
					Filter: &pb.SubscribeRequestFilterAccountsFilter_Datasize{
						Datasize: 408, // byte size of a Position account
					},
				},
			},
		}
		subscription.Accounts[pool] = &accountFilter
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

func processDammV2PoolUpdate(
	ctx context.Context,
	db database.DbPool,
	update *pb.SubscribeUpdateAccount,
) error {
	tx, err := db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	account := solana.PublicKeyFromBytes(update.Account.Pubkey)
	var pool meteora_damm_v2.Pool
	err = bin.NewBorshDecoder(update.Account.Data).Decode(&pool)
	if err != nil {
		return err
	}
	err = upsertDammV2Pool(ctx, tx, update.Slot, account, &pool)
	if err != nil {
		return err
	}
	err = upsertDammV2PoolMetrics(ctx, tx, update.Slot, account, &pool.Metrics)
	if err != nil {
		return err
	}
	err = upsertDammV2PoolFees(ctx, tx, update.Slot, account, &pool.PoolFees)
	if err != nil {
		return err
	}
	err = upsertDammV2PoolBaseFee(ctx, tx, update.Slot, account, &pool.PoolFees.BaseFee)
	if err != nil {
		return err
	}
	err = upsertDammV2PoolDynamicFee(ctx, tx, update.Slot, account, &pool.PoolFees.DynamicFee)
	if err != nil {
		return err
	}
	err = tx.Commit(ctx)
	if err != nil {
		return err
	}
	return nil
}

func processDammV2PositionUpdate(
	ctx context.Context,
	db database.DbPool,
	update *pb.SubscribeUpdateAccount,
) error {
	tx, err := db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	account := solana.PublicKeyFromBytes(update.Account.Pubkey)
	var position meteora_damm_v2.PositionState
	err = bin.NewBorshDecoder(update.Account.Data).Decode(&position)
	if err != nil {
		return err
	}
	err = upsertDammV2Position(ctx, db, update.Slot, account, &position)
	if err != nil {
		return err
	}
	err = upsertDammV2PositionMetrics(ctx, db, update.Slot, account, &position.Metrics)
	if err != nil {
		return err
	}
	err = tx.Commit(ctx)
	if err != nil {
		return err
	}
	return nil
}

// Gets the canonical DAMM V2 pools from the artist coins table.
func getSubscribedDammV2Pools(ctx context.Context, db database.DBTX, limit int, offset int) ([]string, error) {
	sql := `
		SELECT damm_v2_pool
		FROM artist_coins
		WHERE damm_v2_pool IS NOT NULL
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
