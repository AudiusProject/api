package indexer

import (
	"context"
	"fmt"

	"api.audius.co/database"
	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/maypok86/otter"
	pb "github.com/rpcpool/yellowstone-grpc/examples/golang/proto"
	"go.uber.org/zap"
)

type TokenIndexer struct {
	pool       database.DbPool
	grpcConfig GrpcConfig
	rpcClient  RpcClient

	logger *zap.Logger

	// Shared cache for recently fetched transactions
	transactionCache *otter.Cache[solana.Signature, *rpc.GetTransactionResult]
}

const TOKEN_INDEXER_NAME = "token"
const ARTIST_COIN_NOTIFICATION_NAME = "artist_coins_changed"
const MAX_ARTIST_COIN_MINTS_PER_SUBSCRIPTION = 10000
const WORKER_CHANNEL_SIZE = 3000
const WORKER_COUNT = 50

func (t *TokenIndexer) Start(ctx context.Context) {
	// To ensure only one subscription task is running at a time, keep track of
	// the last cancel function and call it on the next notification.
	var lastCancel context.CancelFunc

	// Set up a worker pool for handling messages to keep up with high throughput
	workerChan := make(chan *pb.SubscribeUpdate, WORKER_CHANNEL_SIZE)
	for i := range WORKER_COUNT {
		go func(workerID int) {
			for updateMessage := range workerChan {
				err := t.HandleUpdate(ctx, updateMessage)
				if err != nil {
					t.logger.Error("failed to handle token update", zap.Int("workerID", workerID), zap.Error(err))

					// Add messages that failed to process to the retry queue
					if err := addToRetryQueue(ctx, t.pool, TOKEN_INDEXER_NAME, updateMessage, err.Error()); err != nil {
						t.logger.Error("failed to add to retry queue", zap.Error(err))
					}
				}
			}
		}(i)
	}

	// Ensure all gRPC clients are closed on shutdown and that the workers are closed
	var grpcClients []GrpcClient
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
			t.logger.Warn("context cancelled, not handling update")
			return
		case workerChan <- message:
		}
	}

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

		// Resubscribe to all artist coins
		// TODO: Optimize this to only add/remove new coins instead of resubscribing to all
		clients, err := t.subscribeToArtistCoins(subCtx, handleUpdate)
		grpcClients = clients
		if err != nil {
			t.logger.Error("failed to resubscribe to artist coins", zap.Error(err))
			return
		}

		lastCancel = cancel
	}

	// Initial subscription to all artist coins
	clients, err := t.subscribeToArtistCoins(ctx, handleUpdate)
	if err != nil {
		t.logger.Error("failed to subscribe to artist coins", zap.Error(err))
		return
	}
	grpcClients = clients

	// Watch for new coins to be added
	err = watchPgNotification(ctx, t.pool, ARTIST_COIN_NOTIFICATION_NAME, handleNotif, t.logger)
	if err != nil {
		t.logger.Error("failed to watch for artist coin changes", zap.Error(err))
		return
	}

	// Wait for shutdown
	for {
		select {
		case <-ctx.Done():
			t.logger.Info("received shutdown signal, stopping artist coin indexer")
			return
		default:
		}
	}
}

// Handles a single update message from the gRPC subscription
func (t *TokenIndexer) HandleUpdate(ctx context.Context, msg *pb.SubscribeUpdate) error {
	// Handle slot updates
	slotUpdate := msg.GetSlot()
	if slotUpdate != nil {
		// only update every 10 slots to reduce db load and write latency
		if slotUpdate.Slot%10 == 0 {
			// Use the filter as the checkpoint ID
			checkpointId := msg.Filters[0]

			err := updateCheckpoint(ctx, t.pool, checkpointId, slotUpdate.Slot)
			if err != nil {
				t.logger.Error("failed to update slot checkpoint", zap.Error(err))
			}
		}
	}

	// Handle balance changes
	accUpdate := msg.GetAccount()
	if accUpdate != nil {
		txSig := solana.SignatureFromBytes(accUpdate.Account.TxnSignature)

		// Fetch the transaction details
		txRes, err := fetchTransactionWithCache(ctx, t.transactionCache, t.rpcClient, txSig)
		if err != nil {
			return fmt.Errorf("failed to fetch transaction: %w", err)
		}

		// Decode the transaction
		tx, err := txRes.Transaction.GetTransaction()
		if err != nil {
			return fmt.Errorf("failed to decode transaction: %w", err)
		}

		// Add the lookup table accounts to the message accounts
		tx = resolveLookupTables(ctx, t.rpcClient, tx, txRes.Meta)

		// Extract the mints we're tracking using the subscription's filters
		trackedMints := msg.Filters

		err = processBalanceChanges(ctx, t.pool, accUpdate.Slot, txRes.Meta, tx, txRes.BlockTime.Time(), trackedMints, t.logger)
		if err != nil {
			return fmt.Errorf("failed to process balance changes: %w", err)
		}
	}
	return nil
}

func (t *TokenIndexer) subscribeToArtistCoins(ctx context.Context, handleUpdate func(ctx context.Context, message *pb.SubscribeUpdate)) ([]GrpcClient, error) {
	done := false
	page := 0
	pageSize := MAX_ARTIST_COIN_MINTS_PER_SUBSCRIPTION
	grpcClients := make([]GrpcClient, 0)
	total := 0
	for !done {
		mints, err := getArtistCoins(ctx, t.pool, pageSize, page*pageSize)
		if err != nil {
			return nil, fmt.Errorf("failed to get artist coins: %w", err)
		}
		if len(mints) == 0 {
			t.logger.Info("no more artist coins to subscribe to, exiting")
			return grpcClients, nil
		}
		total += len(mints)
		t.logger.Debug("subscribing to artist coins...", zap.Int("numCoins", len(mints)))
		subscription, err := t.makeMintSubscriptionRequest(ctx, mints)
		if err != nil {
			return nil, fmt.Errorf("failed to make mint subscription request: %w", err)
		}

		grpcClient := NewGrpcClient(t.grpcConfig)
		err = grpcClient.Subscribe(ctx, subscription, handleUpdate, func(err error) {
			t.logger.Error("error in token subscription", zap.Error(err))
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
	t.logger.Info("subscribed to artist coins", zap.Int("numCoins", total))
	return grpcClients, nil
}

func (t *TokenIndexer) makeMintSubscriptionRequest(ctx context.Context, mintAddresses []string) (*pb.SubscribeRequest, error) {
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
	checkpointId, fromSlot, err := ensureCheckpoint(ctx, TOKEN_INDEXER_NAME, t.pool, t.rpcClient, subscription, t.logger)
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

func getArtistCoins(ctx context.Context, db database.DBTX, limit int, offset int) ([]string, error) {
	sqlMints := `SELECT mint FROM artist_coins LIMIT @limit OFFSET @offset`
	rows, err := db.Query(ctx, sqlMints, pgx.NamedArgs{
		"limit":  limit,
		"offset": offset,
	})
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil // No mints found, return empty slice
		}
		return nil, fmt.Errorf("failed to query mints: %w", err)
	}
	mintAddresses, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		return nil, fmt.Errorf("failed to collect mints: %w", err)
	}
	return mintAddresses, nil
}
