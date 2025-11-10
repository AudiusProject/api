package locker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"api.audius.co/database"
	"api.audius.co/solana/indexer/common"
	"api.audius.co/solana/spl/programs/meteora_locker"
	bin "github.com/gagliardetto/binary"
	"github.com/gagliardetto/solana-go"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgxlisten"
	pb "github.com/rpcpool/yellowstone-grpc/examples/golang/proto"
	"go.uber.org/zap"
)

const (
	NAME                       = "LockerIndexer"
	MAX_MINTS_PER_SUBSCRIPTION = 10000 // Arbitrary
	NOTIFICATION_NAME          = "artist_coins_changed"
)

type Indexer struct {
	pool       database.DbPool
	grpcConfig common.GrpcConfig
	rpcClient  common.RpcClient
	logger     *zap.Logger
}

func New(
	grpcConfig common.GrpcConfig,
	rpcClient common.RpcClient,
	pool database.DbPool,
	logger *zap.Logger,
) *Indexer {
	return &Indexer{
		pool:       pool,
		grpcConfig: grpcConfig,
		rpcClient:  rpcClient,
		logger:     logger.Named(NAME),
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

func (d *Indexer) subscribe(ctx context.Context) ([]common.GrpcClient, error) {
	done := false
	page := 0
	pageSize := MAX_MINTS_PER_SUBSCRIPTION
	grpcClients := make([]common.GrpcClient, 0)
	total := 0
	for !done {
		mints, err := common.GetArtistCoinMints(ctx, d.pool, pageSize, page*pageSize)
		if err != nil {
			return nil, fmt.Errorf("failed to get artist coins: %w", err)
		}
		if len(mints) == 0 {
			d.logger.Info("no more artist coins to subscribe to, exiting")
			return grpcClients, nil
		}
		total += len(mints)
		d.logger.Debug("subscribing to artist coins...", zap.Int("count", len(mints)))
		subscription, err := d.makeSubscriptionRequest(ctx, mints)
		if err != nil {
			return nil, fmt.Errorf("failed to make mint subscription request: %w", err)
		}

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

		grpcClient := common.NewGrpcClient(d.grpcConfig)
		err = grpcClient.Subscribe(ctx, subscription, handleMessage, func(err error) {
			d.logger.Error("error in token subscription", zap.Error(err))
		})
		if err != nil {
			return nil, fmt.Errorf("failed to subscribe to artist coins: %w", err)
		}
		grpcClients = append(grpcClients, grpcClient)

		if len(mints) < pageSize {
			done = true
		}
		page++
	}
	d.logger.Info("subscribed to artist coins", zap.Int("count", total))
	return grpcClients, nil
}

// Makes a subscription to the relevant locker accounts and adds slot checkpointing
func (d *Indexer) makeSubscriptionRequest(ctx context.Context, mints []string) (*pb.SubscribeRequest, error) {
	commitment := pb.CommitmentLevel_CONFIRMED
	subscription := &pb.SubscribeRequest{
		Commitment: &commitment,
	}

	// Listen to all lockers
	subscription.Accounts = make(map[string]*pb.SubscribeRequestFilterAccounts)
	for _, mint := range mints {
		accountFilter := pb.SubscribeRequestFilterAccounts{
			Owner: []string{meteora_locker.ProgramID.String()},
			Filters: []*pb.SubscribeRequestFilterAccountsFilter{
				{
					Filter: &pb.SubscribeRequestFilterAccountsFilter_Memcmp{
						Memcmp: &pb.SubscribeRequestFilterAccountsFilterMemcmp{
							Offset: 8 + 32, // Discriminator + recipient pubkey
							Data: &pb.SubscribeRequestFilterAccountsFilterMemcmp_Base58{
								Base58: mint,
							},
						},
					},
				},
			},
		}
		subscription.Accounts[mint] = &accountFilter
	}

	// Ensure this subscription has a checkpoint
	checkpointId, fromSlot, err := common.EnsureCheckpoint(ctx, NAME, d.pool, d.rpcClient, subscription, d.logger)
	if err != nil {
		return nil, fmt.Errorf("failed to set from slot: %w", err)
	}

	// Set the from slot for the subscription
	subscription.FromSlot = &fromSlot

	// Listen for slots for making checkpoints
	subscription.Slots = make(map[string]*pb.SubscribeRequestFilterSlots)
	subscription.Slots[checkpointId] = &pb.SubscribeRequestFilterSlots{}

	return subscription, nil
}

func (d *Indexer) HandleUpdate(ctx context.Context, update *pb.SubscribeUpdate) error {
	// Handle slot updates
	slotUpdate := update.GetSlot()
	if slotUpdate != nil {
		// only update every 10 slots to reduce db load and write latency
		if slotUpdate.Slot%10 == 0 {
			// Use the filter as the checkpoint ID
			checkpointId := update.Filters[0]

			err := common.UpdateCheckpoint(ctx, d.pool, checkpointId, slotUpdate.Slot)
			if err != nil {
				d.logger.Error("failed to update slot checkpoint", zap.Error(err))
			}
		}
	}

	// Handle account updates
	if accountUpdate := update.GetAccount(); accountUpdate != nil {
		err := processLockerAccountUpdate(ctx, d.pool, accountUpdate, d.logger)
		if err != nil {
			return fmt.Errorf("failed to process locker account update: %w", err)
		}
	}
	return nil
}

func processLockerAccountUpdate(
	ctx context.Context,
	db database.DBTX,
	accountUpdate *pb.SubscribeUpdateAccount,
	logger *zap.Logger,
) error {
	account := solana.PublicKeyFromBytes(accountUpdate.Account.Pubkey)

	var escrow meteora_locker.VestingEscrow
	err := bin.NewBorshDecoder(accountUpdate.Account.Data).Decode(&escrow)
	if err != nil {
		return fmt.Errorf("failed to decode locker account %s: %w", account.String(), err)
	}

	err = upsertVestingEscrow(ctx, db, accountUpdate.Slot, account, &escrow)
	if err != nil {
		return fmt.Errorf("failed to upsert locker account %s: %w", account.String(), err)
	}

	logger.Debug("processed locker account update",
		zap.String("account", account.String()),
		zap.String("mint", escrow.TokenMint.String()),
	)

	return nil
}
