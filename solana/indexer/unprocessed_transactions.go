package indexer

import (
	"context"
	"fmt"
	"time"

	"bridgerton.audius.co/database"
	"github.com/gagliardetto/solana-go"
	"github.com/jackc/pgx/v5"
	"go.uber.org/zap"
)

func (s *SolanaIndexer) RetryUnprocessedTransactions(ctx context.Context) error {
	limit := 100
	offset := 0
	logger := s.logger.With(
		zap.String("indexerSource", "retryUnprocessedTransactions"),
	)
	count := 0
	start := time.Now()
	logger.Info("starting retry of unprocessed transactions...")
	for {
		failedTxs, err := getUnprocessedTransactions(ctx, s.pool, limit, offset)
		if err != nil {
			return fmt.Errorf("failed to fetch unprocessed transactions: %w", err)
		}
		if len(failedTxs) == 0 {
			break
		}

		for _, txSig := range failedTxs {
			count++
			err = s.processor.ProcessSignature(ctx, 0, solana.MustSignatureFromBase58(txSig), logger)
			if err != nil {
				logger.Error("failed to process transaction", zap.String("signature", txSig), zap.Error(err))
				offset++
				continue
			}
			logger.Debug("successfully processed transaction", zap.String("signature", txSig))
			deleteUnprocessedTransaction(ctx, s.pool, txSig)
		}
	}
	logger.Info("finished retry of unprocessed transactions",
		zap.Int("count", count),
		zap.Int("failed", offset),
		zap.Duration("duration", time.Since(start)),
	)
	return nil
}

func getUnprocessedTransactions(ctx context.Context, db database.DBTX, limit, offset int) ([]string, error) {
	sql := `SELECT signature FROM sol_unprocessed_txs LIMIT @limit OFFSET @offset;`
	rows, err := db.Query(ctx, sql, pgx.NamedArgs{
		"limit":  limit,
		"offset": offset,
	})
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to query unprocessed transactions: %w", err)
	}
	signatures, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		return nil, fmt.Errorf("failed to collect unprocessed transaction signatures: %w", err)
	}
	return signatures, nil
}

func insertUnprocessedTransaction(ctx context.Context, db database.DBTX, signature, errorMessage string) error {
	sql := `
		INSERT INTO sol_unprocessed_txs (signature, error_message) VALUES (@signature, @error_message) 
		ON CONFLICT (signature) DO UPDATE SET error_message = @error_message, updated_at = NOW()
	;`
	_, err := db.Exec(ctx, sql, pgx.NamedArgs{
		"signature":     signature,
		"error_message": errorMessage,
	})
	if err != nil {
		return fmt.Errorf("failed to insert unprocessed transaction: %w", err)
	}
	return nil
}

func deleteUnprocessedTransaction(ctx context.Context, db database.DBTX, signature string) error {
	sql := `DELETE FROM sol_unprocessed_txs WHERE signature = @signature;`
	_, err := db.Exec(ctx, sql, pgx.NamedArgs{
		"signature": signature,
	})
	if err != nil {
		return fmt.Errorf("failed to delete unprocessed transaction: %w", err)
	}
	return nil
}
