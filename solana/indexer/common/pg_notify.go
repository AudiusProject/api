package common

import (
	"context"
	"errors"
	"fmt"

	"api.audius.co/database"
	"github.com/jackc/pgx/v5/pgconn"
	"go.uber.org/zap"
)

type notificationCallback func(ctx context.Context, notification *pgconn.Notification)

// Listens for a notification and fires a callback when one is received.
// The function spawns a goroutine to listen for notifications, so it returns
// immediately. The caller should ensure the context is cancelled when they want
// to stop listening and wait indefinitely to listen.
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
			childLogger.Debug("received shutdown signal, stopping notification watcher")
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
				if !errors.Is(err, context.Canceled) {
					childLogger.Error("failed waiting for notification", zap.Error(err))
				}
				continue
			}
			if notif == nil {
				childLogger.Warn("received nil notification, continuing to wait for notifications")
				continue
			}

			childLogger.Debug("received notification", zap.String("payload", notif.Payload))
			callback(ctx, notif)
		}
	}()
	return nil
}
