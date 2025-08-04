package indexer

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"bridgerton.audius.co/logging"
	"github.com/gagliardetto/solana-go"
	pb "github.com/rpcpool/yellowstone-grpc/examples/golang/proto"
	"go.uber.org/zap"
)

// LaserStream from Helius only keeps the last 3000 slots.
// Subtract 10 slots to be sure that the subscription doesn't fail.
var MAX_SLOT_GAP = uint64(2990)

type artistCoinsChangedNotification struct {
	Operation string `json:"operation"`
	NewMint   string `json:"new_mint"`
	OldMint   string `json:"old_mint"`
}

func (s *SolanaIndexer) Subscribe(ctx context.Context) error {
	// Set up workers to process updates concurrently
	msgChan := make(chan *pb.SubscribeUpdate, 3000)
	for i := range s.workerCount {
		go func(workerId int32) {
			for msg := range msgChan {
				s.handleMessage(ctx, msg)
			}
		}(i)
	}
	defer close(msgChan)

	// On a new message, queue the message to the worker pool
	onMessage := func(ctx context.Context, msg *pb.SubscribeUpdate) {
		select {
		case <-ctx.Done():
			s.logger.Warn("subscription context cancelled, stopping message processing")
			return
		case msgChan <- msg:
		}
	}

	// Flush the logger every 15 seconds to ensure logs are written out
	go logging.SyncOnTicks(ctx, s.logger, time.Second*15)

	// Acquire a connection to the database and listen for artist coins changes
	conn, err := s.pool.Acquire(ctx)
	if err != nil {
		return fmt.Errorf("failed to acquire database connection: %w", err)
	}
	defer conn.Release()

	rawConn := conn.Conn()
	_, err = rawConn.Exec(ctx, `LISTEN artist_coins_changed`)
	if err != nil {
		return fmt.Errorf("failed to listen for artist coins changes: %w", err)
	}

	// Log when we receive a shutdown signal
	defer func() {
		s.logger.Info("received shutdown signal, stopping subscription")
	}()

	// Loop to reset subscription when the artist coins notification is received
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		coins, err := getArtistCoins(ctx, s.pool, true)
		if err != nil {
			return fmt.Errorf("failed to get artist coins: %w", err)
		}

		subscription, err := buildSubscriptionRequest(coins)
		if err != nil {
			return fmt.Errorf("failed to create subscription: %w", err)
		}

		// Check if a backfill is needed with the new subscription
		// and find the slot to continue from.

		lastIndexedSlot, err := getCheckpointSlot(ctx, s.pool, subscription)
		if err != nil {
			return fmt.Errorf("failed to get last indexed slot: %w", err)
		}

		latestSlot, err := withRetries(func() (uint64, error) {
			return s.rpcClient.GetSlot(ctx, "confirmed")
		}, 5, time.Second*2)
		if err != nil {
			return fmt.Errorf("failed to get slot: %w", err)
		}

		var fromSlot uint64
		minimumSlot := uint64(0)
		if latestSlot > MAX_SLOT_GAP {
			minimumSlot = latestSlot - MAX_SLOT_GAP
		}
		if lastIndexedSlot > minimumSlot {
			fromSlot = lastIndexedSlot
		} else {
			if lastIndexedSlot == 0 {
				s.logger.Warn("no last indexed slot found, starting from minimum slot and skipping backfill", zap.Uint64("fromSlot", minimumSlot))
			} else {
				s.logger.Warn("last indexed slot is too old, starting from minimum slot and backfilling", zap.Uint64("fromSlot", minimumSlot), zap.Uint64("toSlot", lastIndexedSlot))
				go func(lastIndexedSlot, minimumSlot uint64) {
					err := s.Backfill(ctx, lastIndexedSlot, minimumSlot)
					if err != nil {
						s.logger.Error("failed to backfill", zap.Uint64("fromSlot", lastIndexedSlot), zap.Uint64("toSlot", minimumSlot), zap.Error(err))
					}
				}(lastIndexedSlot, minimumSlot)
			}
			fromSlot = minimumSlot
		}

		s.checkpointId, err = insertCheckpointStart(ctx, s.pool, fromSlot, subscription)
		if err != nil {
			return fmt.Errorf("failed to start checkpoint: %w", err)
		}

		subscription.FromSlot = &fromSlot

		subCtx, cancel := context.WithCancel(ctx)
		defer cancel()

		if err := s.grpcClient.Subscribe(subCtx, subscription, onMessage, s.onError); err != nil {
			return fmt.Errorf("failed to subscribe to gRPC server: %w", err)
		}

		s.logger.Info("Solana indexer subscribed and listening...", zap.Uint64("fromSlot", fromSlot))

		for {
			notif, err := rawConn.WaitForNotification(ctx)
			if err != nil {
				return fmt.Errorf("failed to wait for notification: %w", err)
			}

			if notif == nil {
				s.logger.Warn("received nil notification, continuing to wait for artist_coins changes")
				continue
			}
			if strings.HasPrefix(notif.Channel, "artist_coins_changed") {
				var notifData artistCoinsChangedNotification
				err := json.Unmarshal([]byte(notif.Payload), &notifData)
				if err != nil {
					s.logger.Error("failed to unmarshal artist_coins changed notification", zap.Error(err))
					continue
				}
				if notifData.Operation != "INSERT" && notifData.Operation != "DELETE" {
					// ignore updates - only care if mints are added or removed
					continue
				}
				s.logger.Info("artist_coins changed, re-starting subscription",
					zap.String("oldMint", notifData.OldMint),
					zap.String("newMint", notifData.NewMint),
					zap.String("operation", notifData.Operation))
				cancel()
				s.grpcClient.Close()
				<-subCtx.Done()
				break
			}
		}
	}
}

func buildSubscriptionRequest(mintAddresses []string) (*pb.SubscribeRequest, error) {
	commitment := pb.CommitmentLevel_CONFIRMED
	subscription := &pb.SubscribeRequest{
		Commitment: &commitment,
	}

	// Listen for slots for making checkpoints
	subscription.Slots = make(map[string]*pb.SubscribeRequestFilterSlots)
	subscription.Slots["checkpoints"] = &pb.SubscribeRequestFilterSlots{}

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
							Offset: 0,
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

	// Listen to all the Audius programs for transactions (currently redundant)
	// programs := []string{
	// 	claimable_tokens.ProgramID.String(),
	// 	reward_manager.ProgramID.String(),
	// 	payment_router.ProgramID.String(),
	// }
	// vote := false
	// failed := false
	// subscription.Transactions = make(map[string]*pb.SubscribeRequestFilterTransactions)
	// transactionFilter := pb.SubscribeRequestFilterTransactions{
	// 	Vote:           &vote,
	// 	Failed:         &failed,
	// 	AccountInclude: programs,
	// }
	// subscription.Transactions["audiusPrograms"] = &transactionFilter

	return subscription, nil
}

// Handles a message from the gRPC subscription.
func (s *SolanaIndexer) handleMessage(ctx context.Context, msg *pb.SubscribeUpdate) {
	logger := s.logger.With(zap.String("indexerSource", "grpc"))

	if slotUpdate := msg.GetSlot(); slotUpdate != nil && slotUpdate.Slot > 0 {
		// only update every 10 slots to reduce db load and write latency
		if slotUpdate.Slot%10 == 0 {
			s.logger.Debug("slot update", zap.Uint64("slot", slotUpdate.Slot))
			err := updateCheckpoint(ctx, s.pool, s.checkpointId, slotUpdate.Slot)
			if err != nil {
				logger.Error("failed to update slot checkpoint", zap.Error(err))
			}
		}
	}

	accUpdate := msg.GetAccount()
	if accUpdate != nil {
		txSig := solana.SignatureFromBytes(accUpdate.Account.TxnSignature)
		err := s.processor.ProcessSignature(ctx, accUpdate.Slot, txSig, logger)
		if err != nil {
			logger.Error("failed to process signature", zap.Error(err))
		}
	}
}

func (s *SolanaIndexer) onError(err error) {
	s.logger.Error("error in solana indexer", zap.Error(err))
}
