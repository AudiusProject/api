package common

import (
	"context"
	"fmt"
	"time"

	"api.audius.co/database"
	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/maypok86/otter"
	"go.uber.org/zap"
)

func WithRetries(f func() error, maxRetries int, interval time.Duration) error {
	err := f()
	retries := 0
	for err != nil && retries < maxRetries {
		time.Sleep(interval)
		err = f()
		retries++
	}
	if err != nil {
		return fmt.Errorf("retry failed: %w", err)
	}
	return nil
}

func WithRetriesResult[T any](f func() (T, error), maxRetries int, interval time.Duration) (T, error) {
	result, err := f()
	retries := 0
	for err != nil && retries < maxRetries {
		time.Sleep(interval)
		result, err = f()
		retries++
	}
	if err != nil {
		var zero T
		return zero, fmt.Errorf("retry failed: %w", err)
	}
	return result, nil
}

type notificationCallback func(ctx context.Context, notification *pgconn.Notification)

func WatchPgNotification(ctx context.Context, pool database.DbPool, notification string, callback notificationCallback, logger *zap.Logger) error {
	if logger == nil {
		logger = zap.NewNop()
	}

	childLogger := logger.With(zap.String("notification", notification))

	conn, err := pool.Acquire(ctx)
	if err != nil {
		return fmt.Errorf("failed to acquire database connection: %w", err)
	}

	rawConn := conn.Conn()
	_, err = rawConn.Exec(ctx, fmt.Sprintf(`LISTEN %s`, notification))
	if err != nil {
		return fmt.Errorf("failed to listen for %s changes: %w", notification, err)
	}

	go func() {
		defer func() {
			if rawConn != nil && !rawConn.PgConn().IsClosed() && ctx.Err() != nil {
				_, _ = rawConn.Exec(ctx, fmt.Sprintf(`UNLISTEN %s`, notification))
			}
			childLogger.Info("received shutdown signal, stopping notification watcher")
			conn.Release()
		}()
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}

			notif, err := rawConn.WaitForNotification(ctx)
			if err != nil {
				childLogger.Error("failed waiting for notification", zap.Error(err))
			}
			if notif == nil {
				childLogger.Warn("received nil notification, continuing to wait for notifications")
				continue
			}
			callback(ctx, notif)
		}
	}()
	return nil
}

// Gets a transaction from a cache or fetches it from the RPC. Handles retries.
func FetchTransactionWithCache(
	ctx context.Context,
	transactionCache *otter.Cache[solana.Signature,
		*rpc.GetTransactionResult],
	rpcClient RpcClient,
	signature solana.Signature,
) (*rpc.GetTransactionResult, error) {
	// Check if the transaction is in the cache
	if transactionCache != nil {
		if res, ok := transactionCache.Get(signature); ok {
			return res, nil
		}
	}

	// If the transaction is not in the cache, fetch it from the RPC
	res, err := WithRetriesResult(func() (*rpc.GetTransactionResult, error) {
		return rpcClient.GetTransaction(
			ctx,
			signature,
			&rpc.GetTransactionOpts{
				Commitment:                     rpc.CommitmentConfirmed,
				MaxSupportedTransactionVersion: &rpc.MaxSupportedTransactionVersion0,
			},
		)
	}, 5, 1*time.Second)
	if err != nil {
		return nil, fmt.Errorf("failed to get transaction: %w", err)
	}

	// Store the fetched transaction in the cache
	if transactionCache != nil {
		transactionCache.Set(signature, res)
	}

	return res, nil
}

// Resolves address lookup tables in the given transaction using the provided metadata.
func ResolveLookupTables(
	ctx context.Context,
	rpcClient RpcClient,
	tx *solana.Transaction,
	meta *rpc.TransactionMeta,
) *solana.Transaction {
	addressTables := make(map[solana.PublicKey]solana.PublicKeySlice)
	writablePos := 0
	readonlyPos := 0
	for _, lu := range tx.Message.AddressTableLookups {
		addresses := make(solana.PublicKeySlice, 256)
		for _, idx := range lu.WritableIndexes {
			addresses[idx] = meta.LoadedAddresses.Writable[writablePos]
			writablePos += 1
		}
		for _, idx := range lu.ReadonlyIndexes {
			addresses[idx] = meta.LoadedAddresses.ReadOnly[readonlyPos]
			readonlyPos += 1
		}
		addressTables[lu.AccountKey] = addresses
	}
	tx.Message.SetAddressTables(addressTables)
	return tx
}
