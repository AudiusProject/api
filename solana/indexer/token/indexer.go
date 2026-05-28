package token

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"api.audius.co/database"
	"api.audius.co/solana/indexer/common"
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
	NAME                       = "TokenIndexer"
	NOTIFICATION_NAME          = "artist_coins_mint_changed"
	MAX_MINTS_PER_SUBSCRIPTION = 10000 // Arbitrary
	WORKER_CHANNEL_SIZE        = 3000
	WORKER_COUNT               = 50
)

type Indexer struct {
	pool       database.DbPool
	grpcConfig common.GrpcConfig
	rpcClient  common.RpcClient

	logger *zap.Logger

	// Shared cache for recently fetched transactions
	transactionCache *otter.Cache[solana.Signature, *rpc.GetTransactionResult]
}

func New(
	config common.GrpcConfig,
	rpcClient common.RpcClient,
	pool database.DbPool,
	transactionCache *otter.Cache[solana.Signature, *rpc.GetTransactionResult],
	logger *zap.Logger,
) *Indexer {
	return &Indexer{
		pool:             pool,
		grpcConfig:       config,
		rpcClient:        rpcClient,
		transactionCache: transactionCache,
		logger:           logger.Named(NAME),
	}
}

func (d *Indexer) Start(ctx context.Context) {
	// To ensure only one subscription task is running at a time, keep track of
	// the last cancel function and call it on the next notification.
	var lastCancel context.CancelFunc

	// Set up a worker pool for handling messages to keep up with high throughput
	workerChan := make(chan *pb.SubscribeUpdate, WORKER_CHANNEL_SIZE)
	for i := range WORKER_COUNT {
		go func(workerID int) {
			for updateMessage := range workerChan {
				err := d.HandleUpdate(ctx, updateMessage)
				if err != nil {
					d.logger.Error("failed to handle token update", zap.Int("workerID", workerID), zap.Error(err))

					// Add messages that failed to process to the retry queue
					if err := common.AddToRetryQueue(ctx, d.pool, NAME, updateMessage, err.Error()); err != nil {
						d.logger.Error("failed to add to retry queue", zap.Error(err))
					}
				}
			}
		}(i)
	}

	// Ensure all gRPC clients are closed on shutdown and that the workers are closed
	var grpcClients []common.GrpcClient
	defer (func() {
		for _, client := range grpcClients {
			client.Close()
		}
		close(workerChan)
	})()

	// Post messages to the worker pool
	handleUpdate := func(ctx context.Context, message *pb.SubscribeUpdate) {
		select {
		case <-ctx.Done():
			d.logger.Warn("context cancelled, not handling update")
			return
		case workerChan <- message:
		}
	}

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
			d.logger.Info("resubscribing due to mint change",
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

		// Resubscribe to all artist coins
		// TODO: Optimize this to only add/remove coins instead of resubscribing to all
		clients, err := d.subscribeToArtistCoins(subCtx, handleUpdate)
		grpcClients = clients
		if err != nil {
			cancel()
			return fmt.Errorf("failed to resubscribe to artist coins: %w", err)
		}

		lastCancel = cancel
		return nil
	}

	// Initial subscription to all artist coins
	clients, err := d.subscribeToArtistCoins(ctx, handleUpdate)
	if err != nil {
		d.logger.Error("failed to subscribe to artist coins", zap.Error(err))
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

	// Handle balance changes
	accUpdate := msg.GetAccount()
	if accUpdate != nil {
		txSig := solana.SignatureFromBytes(accUpdate.Account.TxnSignature)

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

		// Extract the mints we're tracking using the subscription's filters
		trackedMints := msg.Filters

		// Process balance changes for this subscription's mints
		logger := d.logger.With(
			zap.String("signature", txSig.String()),
			zap.Uint64("slot", txRes.Slot),
		)
		err = common.ProcessBalanceChanges(ctx, d.pool, txRes.Slot, txRes.Meta, tx, txRes.BlockTime.Time(), trackedMints, logger)
		if err != nil {
			return fmt.Errorf("failed to process balance changes: %w", err)
		}

		// Detect AUDIO top-up memos (Stripe / Coinbase Pay / Unknown) and tag
		// the signature in sol_transfer_memo_types so
		// v_token_transactions_history surfaces transaction_type as the right
		// purchase_* value instead of a bare transfer. Cheap: scans
		// instructions once, skips non-memo programs immediately. Idempotent
		// via ON CONFLICT.
		if memoType, instIdx, ok := findAudioVendorMemoType(tx); ok {
			if err := insertAudioVendorMemoMarker(ctx, d.pool, txSig.String(), instIdx, txRes.Slot, memoType); err != nil {
				logger.Warn("failed to insert audio vendor memo marker", zap.Error(err))
			}
		}
	}
	return nil
}

func (d *Indexer) subscribeToArtistCoins(ctx context.Context, handleUpdate func(ctx context.Context, message *pb.SubscribeUpdate)) ([]common.GrpcClient, error) {
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
		subscription, err := d.makeMintSubscriptionRequest(ctx, mints)
		if err != nil {
			return nil, fmt.Errorf("failed to make mint subscription request: %w", err)
		}

		grpcClient := common.NewGrpcClient(d.grpcConfig)
		err = grpcClient.Subscribe(ctx, subscription, handleUpdate, func(err error) {
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

func (t *Indexer) makeMintSubscriptionRequest(ctx context.Context, mintAddresses []string) (*pb.SubscribeRequest, error) {
	commitment := pb.CommitmentLevel_CONFIRMED
	subscription := &pb.SubscribeRequest{
		Commitment: &commitment,
	}

	// Listen to all the token accounts for the mints we care about
	subscription.Accounts = make(map[string]*pb.SubscribeRequestFilterAccounts)
	for _, mint := range mintAddresses {
		accountFilter := pb.SubscribeRequestFilterAccounts{
			Owner: []string{solana.TokenProgramID.String()},
			Filters: []*pb.SubscribeRequestFilterAccountsFilter{
				{
					Filter: &pb.SubscribeRequestFilterAccountsFilter_TokenAccountState{
						TokenAccountState: true,
					},
				},
				{
					Filter: &pb.SubscribeRequestFilterAccountsFilter_Memcmp{
						Memcmp: &pb.SubscribeRequestFilterAccountsFilterMemcmp{
							Offset: 0, // Mint is at offset 0
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
	checkpointId, fromSlot, err := common.EnsureCheckpoint(ctx, NAME, t.pool, t.rpcClient, subscription, t.logger)
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
