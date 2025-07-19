package indexer

import (
	"context"
	"fmt"
	"sync"
	"time"

	"bridgerton.audius.co/database"
	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
	"github.com/jackc/pgx/v5"
	"go.uber.org/zap"
)

var BATCH_DELAY_MS = uint64(50)
var TRANSACTION_DELAY_MS = uint(5)

func (s *SolanaIndexer) Backfill(ctx context.Context, fromSlot uint64, toSlot uint64) error {
	txRange, err := getTransactionRange(ctx, s.pool, fromSlot, toSlot)
	if err != nil {
		return fmt.Errorf("failed to get transaction range: %w", err)
	}

	if txRange.before.IsZero() {
		block, err := s.rpcClient.GetBlockWithOpts(ctx, toSlot, &rpc.GetBlockOpts{
			TransactionDetails:             rpc.TransactionDetailsSignatures,
			Commitment:                     rpc.CommitmentConfirmed,
			MaxSupportedTransactionVersion: &rpc.MaxSupportedTransactionVersion0,
		})
		if err != nil || len(block.Signatures) == 0 {
			return fmt.Errorf("failed to get block: %w", err)
		}
		txRange.before = block.Signatures[len(block.Signatures)-1]
	}

	if txRange.until.IsZero() {
		block, err := s.rpcClient.GetBlockWithOpts(ctx, fromSlot, &rpc.GetBlockOpts{
			TransactionDetails:             rpc.TransactionDetailsSignatures,
			Commitment:                     rpc.CommitmentConfirmed,
			MaxSupportedTransactionVersion: &rpc.MaxSupportedTransactionVersion0,
		})
		if err != nil || len(block.Signatures) == 0 {
			return fmt.Errorf("failed to get block: %w", err)
		}
		txRange.until = block.Signatures[0]
	}

	var wg sync.WaitGroup
	for _, address := range []solana.PublicKey{
		s.config.SolanaConfig.RewardManagerProgramID,
		s.config.SolanaConfig.ClaimableTokensProgramID,
		s.config.SolanaConfig.PaymentRouterProgramID,
	} {
		wg.Add(1)
		go func(address solana.PublicKey) {
			defer wg.Done()
			s.backfillAddressTransactions(ctx, address, txRange, fromSlot, toSlot)
		}(address)
	}
	wg.Wait()

	return nil
}

// Fetches and processes transactions for a given address within a given signature/slot range.
func (s *SolanaIndexer) backfillAddressTransactions(ctx context.Context, address solana.PublicKey, txRange transactionRange, fromSlot uint64, toSlot uint64) {
	var lastIndexedSig solana.Signature
	foundIntersection := false
	before := txRange.before

	logger := s.logger.With(
		zap.String("indexerSource", "rpcBackfill"),
		zap.String("address", address.String()),
	)
	logger.Info("starting backfill for address",
		zap.String("before", txRange.before.String()),
		zap.String("until", txRange.until.String()),
		zap.Uint64("fromSlot", fromSlot),
		zap.Uint64("toSlot", toSlot),
	)

	limit := 1000
	opts := rpc.GetSignaturesForAddressOpts{
		Commitment:     rpc.CommitmentConfirmed,
		Until:          txRange.until,
		Limit:          &limit,
		MinContextSlot: &toSlot,
	}

	for !foundIntersection {
		select {
		case <-ctx.Done():
			logger.Error("failed to find intersection", zap.Error(ctx.Err()))
			return
		default:
		}

		opts.Before = before
		res, err := withRetries(func() ([]*rpc.TransactionSignature, error) {
			return s.rpcClient.GetSignaturesForAddressWithOpts(ctx, address, &opts)
		}, 5, time.Second*1)
		if err != nil {
			logger.Error("failed to get signatures for address", zap.Error(err))
			continue
		}

		if len(res) == 0 {
			logger.Info("no transactions left to index.")
			break
		}

		for _, sig := range res {
			logger := logger.With(zap.String("signature", sig.Signature.String()))

			select {
			case <-ctx.Done():
				logger.Error("failed to process signature", zap.Error(ctx.Err()))
				return
			default:
			}

			if sig.Slot < fromSlot {
				foundIntersection = true
				logger.Info("found intersection with transaction range")
				break
			}

			// Skip error transactions
			if sig.Err != nil {
				lastIndexedSig = sig.Signature
				continue
			}

			// Skip zero signatures
			if sig.Signature.IsZero() {
				continue
			}

			// Skip existing transactions
			var exists bool
			checkSql := `SELECT EXISTS(SELECT 1 FROM sol_token_account_balance_changes WHERE signature = @signature)`
			err := s.pool.QueryRow(context.Background(), checkSql, pgx.NamedArgs{
				"signature": sig.Signature,
			}).Scan(&exists)

			if err != nil {
				logger.Error("failed to check if signature exists", zap.Error(err))
				continue
			}
			if exists {
				lastIndexedSig = sig.Signature
				continue
			}

			err = s.ProcessSignature(ctx, sig.Slot, sig.Signature, logger)
			if err != nil {
				logger.Error("failed to process signature", zap.Error(err))
			}

			lastIndexedSig = sig.Signature

			// sleep for a bit to avoid hitting rate limits
			time.Sleep(time.Millisecond * time.Duration(TRANSACTION_DELAY_MS))
		}

		before = lastIndexedSig
		logger.Info("finished transaction batch",
			zap.Int("count", len(res)),
		)

		// sleep for a bit to avoid hitting rate limits
		time.Sleep(time.Millisecond * time.Duration(BATCH_DELAY_MS))
	}
	insertBackfillCheckpoint(ctx, s.pool, fromSlot, toSlot, address.String())
	logger.Info("backfill completed")
}

type transactionRangeRow struct {
	Before *string
	Until  *string
}

type transactionRange struct {
	before solana.Signature
	until  solana.Signature
}

func getTransactionRange(ctx context.Context, db database.DBTX, fromSlot uint64, toSlot uint64) (transactionRange, error) {
	sql := `
		SELECT
			(
				SELECT signature
				FROM sol_token_account_balance_changes
				WHERE slot <= @fromSlot
				ORDER BY slot DESC
				LIMIT 1
			) AS until,
			(
				SELECT signature
				FROM sol_token_account_balance_changes
				WHERE slot >= @toSlot
				ORDER BY slot ASC
				LIMIT 1
			) AS before
	;`

	rows, err := db.Query(ctx, sql, pgx.NamedArgs{
		"fromSlot": fromSlot,
		"toSlot":   toSlot,
	})
	if err != nil {
		return transactionRange{}, fmt.Errorf("failed to query transaction range: %w", err)
	}

	rangeResult, err := pgx.CollectExactlyOneRow(rows, pgx.RowToStructByName[transactionRangeRow])
	if err != nil {
		return transactionRange{}, fmt.Errorf("failed to collect transaction range row: %w", err)
	}

	res := transactionRange{
		before: solana.Signature{},
		until:  solana.Signature{},
	}
	if rangeResult.Before != nil {
		res.before, err = solana.SignatureFromBase58(*rangeResult.Before)
		if err != nil {
			return transactionRange{}, fmt.Errorf("failed to parse before signature: %w", err)
		}
	}

	if rangeResult.Until != nil {
		res.until, err = solana.SignatureFromBase58(*rangeResult.Until)
		if err != nil {
			return transactionRange{}, fmt.Errorf("failed to parse until signature: %w", err)
		}
	}

	return res, nil
}
