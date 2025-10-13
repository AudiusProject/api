package indexer

import (
	"context"
	"fmt"
	"time"

	"api.audius.co/database"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"go.uber.org/zap"
)

func withRetries(f func() error, maxRetries int, interval time.Duration) error {
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

func withRetriesResult[T any](f func() (T, error), maxRetries int, interval time.Duration) (T, error) {
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

var mintsCache []string

func getArtistCoins(ctx context.Context, db database.DBTX, forceRefresh bool) ([]string, error) {
	if !forceRefresh && mintsCache != nil {
		return mintsCache, nil
	}
	sqlMints := `SELECT mint FROM artist_coins`
	rows, err := db.Query(ctx, sqlMints)
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
	mintsCache = mintAddresses
	return mintAddresses, nil
}

type notificationCallback func(ctx context.Context, notification *pgconn.Notification)

func watchPgNotification(ctx context.Context, pool database.DbPool, notification string, callback notificationCallback, logger *zap.Logger) error {
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
