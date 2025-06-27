package indexers

import (
	"context"
	"strconv"
	"sync"
	"time"

	"bridgerton.audius.co/config"
	"bridgerton.audius.co/logging"
	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/mr-tron/base58"
	pb "github.com/rpcpool/yellowstone-grpc/examples/golang/proto"
	"go.uber.org/zap"
)

type SolanaIndexer struct {
	grpcClient *GrpcClient
	rpcClient  *rpc.Client
	logger     *zap.Logger

	addressesToWatch []solana.PublicKey
	tokenMints       []solana.PublicKey
}

// LaserStream from Helius only keeps the last 3000 slots
var MAX_SLOT_GAP = uint64(3000)

var BATCH_DELAY_MS = uint64(50)
var TRANSACTION_DELAY_MS = uint(5)

// Creates a Solana indexer.
func NewSolanaIndexer(config config.Config) *SolanaIndexer {
	logger := logging.NewZapLogger(config).
		With(zap.String("service", "SolanaIndexer"))

	grpcClient, err := NewGrpcClient(GrpcConfig{
		Server:               config.SolanaConfig.GrpcProvider,
		ApiToken:             config.SolanaConfig.GrpcToken,
		MaxReconnectAttempts: 5,
		WriteDbUrl:           config.WriteDbUrl,
	})
	if err != nil {
		logger.Fatal("failed to create gRPC client", zap.Error(err))
	}

	rpcClient := rpc.New(config.SolanaConfig.RpcProviders[0])

	return &SolanaIndexer{
		grpcClient: grpcClient,
		rpcClient:  rpcClient,
		logger:     logger,
		tokenMints: []solana.PublicKey{
			config.SolanaConfig.MintAudio,
		},
		addressesToWatch: []solana.PublicKey{
			config.SolanaConfig.MintAudio,
			config.SolanaConfig.RewardManagerProgramID,
			config.SolanaConfig.ClaimableTokensProgramID,
		},
	}
}

// Starts the indexer.
func (s *SolanaIndexer) Start(ctx context.Context) {
	latestChainSlot, err := s.rpcClient.GetSlot(ctx, rpc.CommitmentConfirmed)
	for err != nil {
		s.logger.Error("failed to get latest chain slot - retrying", zap.Error(err))
		time.Sleep(time.Second * 5)
		latestChainSlot, err = s.rpcClient.GetSlot(ctx, rpc.CommitmentConfirmed)
	}

	getLastIndexedSlotSql := `SELECT slot FROM solana_indexer_checkpoint LIMIT 1`
	var lastIndexedSlot uint64
	if err := s.grpcClient.pool.QueryRow(ctx, getLastIndexedSlotSql).Scan(&lastIndexedSlot); err != nil {
		if err != pgx.ErrNoRows {
			s.logger.Error("error getting last indexed slot", zap.Error(err))
		}
	}

	// LaserStream has a max slot gap of 3000 slots. Backfill the indexer until
	// the latest chain slot is within MAX_SLOT_GAP of the last indexed slot,
	// then start the gRPC subscription from the last indexed slot.
	for latestChainSlot-lastIndexedSlot > MAX_SLOT_GAP {
		s.logger.Info("slot gap too large, backfilling indexer", zap.Uint64("latest_chain_slot", latestChainSlot), zap.Uint64("last_indexed_slot", lastIndexedSlot))
		var wg sync.WaitGroup
		for _, address := range s.addressesToWatch {
			wg.Add(1)
			go func(addr solana.PublicKey) {
				defer wg.Done()
				s.backfillAddressTransactions(ctx, addr, lastIndexedSlot)
			}(address)
		}
		wg.Wait()
		lastIndexedSlot = latestChainSlot
		latestChainSlot, err = s.rpcClient.GetSlot(ctx, rpc.CommitmentConfirmed)
		for err != nil {
			s.logger.Error("failed to get latest chain slot - retrying", zap.Error(err))
			time.Sleep(time.Second * 5)
			latestChainSlot, err = s.rpcClient.GetSlot(ctx, rpc.CommitmentConfirmed)
		}
	}

	commitment := pb.CommitmentLevel_CONFIRMED
	subscription := &pb.SubscribeRequest{
		Commitment: &commitment,
		FromSlot:   &lastIndexedSlot,
	}
	subscription.Transactions = make(map[string]*pb.SubscribeRequestFilterTransactions, 0)
	vote := false
	failed := false
	accounts := make([]string, len(s.addressesToWatch))
	for i, address := range s.addressesToWatch {
		accounts[i] = address.String()
	}
	tokenFilter := pb.SubscribeRequestFilterTransactions{
		Vote:           &vote,
		AccountInclude: accounts,
		Failed:         &failed,
	}
	subscription.Transactions["tokenIndexer"] = &tokenFilter
	if err := s.grpcClient.Subscribe(subscription, s.onMessage, s.onError); err != nil {
		s.logger.Error("error subscribing to gRPC server", zap.Error(err))
		return
	}

	s.logger.Info("listening for new transactions...")
}

// Handles a message from the gRPC subscription.
func (s *SolanaIndexer) onMessage(msg *pb.SubscribeUpdate) {
	transaction := msg.GetTransaction()
	if transaction != nil && transaction.Transaction.Meta.Err == nil {
		tx := transaction.Transaction

		logger := s.logger.With(
			zap.String("signature", base58.Encode(tx.Signature)),
			zap.String("indexerSource", "grpc"),
		)
		logger.Debug("received transaction")

		balanceChanges, err := getTokenBalanceChanges(&geyserTransactionAdapter{tx: tx})
		if err != nil {
			logger.Error("failed to get token balance changes", zap.Error(err))
			return
		}

		for acc, balanceChange := range balanceChanges {
			for _, mint := range s.tokenMints {
				// Ignore untracked mints
				if balanceChange.Mint != mint.String() {
					continue
				}
				err = insertBalanceChange(context.Background(), s.grpcClient.pool, balanceChangeRow{
					balanceChange: balanceChange,
					account:       acc,
					signature:     base58.Encode(tx.Signature),
					slot:          transaction.Slot,
				}, logger)
				if err != nil {
					logger.Error("failed to insert token transaction", zap.Error(err))
					return
				}
			}

		}
	}
}

type balanceChangeRow struct {
	balanceChange *BalanceChange
	account       string
	signature     string
	slot          uint64
}
type dbExecutor interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

func insertBalanceChange(ctx context.Context, db dbExecutor, row balanceChangeRow, logger *zap.Logger) error {
	sql := `INSERT INTO solana_token_txs (account_address, mint, change, balance, signature, slot)
						VALUES (@account_address, @mint, @change, @balance, @signature, @slot)
						ON CONFLICT DO NOTHING`
	_, err := db.Exec(ctx, sql, pgx.NamedArgs{
		"account_address": row.account,
		"mint":            row.balanceChange.Mint,
		"change":          row.balanceChange.Change,
		"balance":         row.balanceChange.PostTokenBalance,
		"signature":       row.signature,
		"slot":            row.slot,
	})
	if logger != nil {
		logger.Debug("inserting balance change...",
			zap.String("account", row.account),
			zap.String("mint", row.balanceChange.Mint),
			zap.Uint64("balance", row.balanceChange.PostTokenBalance),
			zap.Int64("change", row.balanceChange.Change),
			zap.Uint64("slot", row.slot),
		)
	}
	return err
}

func (s *SolanaIndexer) onError(err error) {
	s.logger.Error("Error in solana indexer", zap.Error(err))
}

// Fetches and processes transactions for a given address.
// It should not update the last indexed slot. The caller will do that once
// backfill completes as the last indexed slot is for all backfillers and
// backfilling works in reverese chronological order.
func (s *SolanaIndexer) backfillAddressTransactions(ctx context.Context, address solana.PublicKey, lastIndexedSlot uint64) {
	var before solana.Signature
	var lastIndexedSig solana.Signature
	foundIntersection := false
	logger := s.logger.With(
		zap.String("indexerSource", "rpcBackfill"),
		zap.String("address", address.String()),
	)
	logger.Info("starting backfill for address")
	limit := 1000
	for !foundIntersection {
		res, err := s.rpcClient.GetSignaturesForAddressWithOpts(
			ctx,
			address,
			&rpc.GetSignaturesForAddressOpts{
				Commitment: rpc.CommitmentConfirmed,
				Before:     before,
				Limit:      &limit,
			},
		)
		if err != nil {
			logger.Error("failed to get signatures for address", zap.Error(err))
			continue
		}
		if len(res) == 0 {
			return
		}

		tx, err := s.grpcClient.pool.Begin(ctx)
		defer tx.Rollback(ctx)
		if err != nil {
			logger.Error("failed to begin transaction", zap.Error(err))
			continue
		}
		for _, sig := range res {
			if sig.Slot < lastIndexedSlot {
				foundIntersection = true
				break
			}
			logger := logger.With(zap.String("signature", sig.Signature.String()))

			// Skip error transactions
			if sig.Err != nil {
				lastIndexedSig = sig.Signature
				continue
			}

			// Skip zero signatures
			if sig.Signature.IsZero() {
				continue
			}

			// Check if the signature already exists in the solana_token_txs table
			// and skip it if it does
			var exists bool
			checkSql := `SELECT EXISTS(SELECT 1 FROM solana_token_txs WHERE signature = @signature)`
			err := s.grpcClient.pool.QueryRow(context.Background(), checkSql, pgx.NamedArgs{
				"signature": sig.Signature,
			}).Scan(&exists)

			if err != nil {
				logger.Error("failed to check if signature exists", zap.Error(err))
				break
			}

			// Skip existing transactions
			if exists {
				lastIndexedSig = sig.Signature
				continue
			}

			txRes, err := s.rpcClient.GetParsedTransaction(
				ctx,
				sig.Signature,
				&rpc.GetParsedTransactionOpts{
					Commitment:                     rpc.CommitmentConfirmed,
					MaxSupportedTransactionVersion: &rpc.MaxSupportedTransactionVersion0,
				},
			)
			if err != nil {
				logger.Error("failed to get transaction", zap.Error(err))
				break
			}

			balanceChanges, err := getTokenBalanceChanges(&rpcTransactionAdapter{tx: txRes})
			if err != nil {
				logger.Error("failed to get balance changes", zap.Error(err))
				break
			}
			for acc, balanceChange := range balanceChanges {
				for _, mint := range s.tokenMints {
					// Ignore untracked mints
					if balanceChange.Mint != mint.String() {
						continue
					}

					err = insertBalanceChange(ctx, tx, balanceChangeRow{
						balanceChange: balanceChange,
						account:       acc,
						signature:     sig.Signature.String(),
						slot:          txRes.Slot,
					}, logger)
					if err != nil {
						logger.Error("failed to insert token transaction", zap.Error(err))
						break
					}
				}
			}
			lastIndexedSig = sig.Signature

			// sleep for a bit to avoid hitting rate limits
			time.Sleep(time.Millisecond * time.Duration(TRANSACTION_DELAY_MS))
		}

		err = tx.Commit(ctx)
		if err != nil {
			logger.Error("failed to commit transaction batch",
				zap.Error(err),
				zap.Int("count", len(res)),
				zap.String("before", before.String()),
			)
		} else {
			before = lastIndexedSig
			logger.Info("committed transaction batch",
				zap.Int("count", len(res)),
				zap.String("before", before.String()),
			)
		}
		// sleep for a bit to avoid hitting rate limits
		time.Sleep(time.Millisecond * time.Duration(BATCH_DELAY_MS))
	}
	logger.Info("backfill completed")
}

type BalanceChange struct {
	Mint             string
	PreTokenBalance  uint64
	PostTokenBalance uint64
	Change           int64
}

// Gets a map of account address to balance change from the given transaction.
func getTokenBalanceChanges(tx transactionAdapter) (map[string]*BalanceChange, error) {
	balanceChanges := make(map[string]*BalanceChange)

	// Make a list of all accounts involved in the transaction
	allAccounts := tx.GetAllAccountKeys()
	// Pre balances
	for _, balance := range tx.GetPreTokenBalances() {
		acc := allAccounts[balance.GetAccountIndex()]
		preBalance, err := strconv.ParseUint(balance.GetUiTokenAmount().GetAmount(), 10, 64)
		if err != nil {
			return balanceChanges, err
		}

		balanceChanges[acc] = &BalanceChange{
			Mint:            balance.GetMint(),
			PreTokenBalance: preBalance,
		}
	}

	// Post balances and changes
	for _, balance := range tx.GetPostTokenBalances() {
		acc := allAccounts[balance.GetAccountIndex()]
		postBalance, err := strconv.ParseUint(balance.GetUiTokenAmount().GetAmount(), 10, 64)
		if err != nil {
			return balanceChanges, err
		}

		b := balanceChanges[acc]
		if b == nil {
			b = &BalanceChange{
				Mint:            balance.GetMint(),
				PreTokenBalance: 0,
			}
			balanceChanges[acc] = b
		}
		b.PostTokenBalance = postBalance
		b.Change = int64(b.PostTokenBalance) - int64(b.PreTokenBalance)
	}
	return balanceChanges, nil
}
