package common

import (
	"context"
	"testing"
	"time"

	"api.audius.co/database"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/require"
	"github.com/test-go/testify/assert"
	"go.uber.org/zap"
)

func TestWatchNotification(t *testing.T) {
	pool := database.CreateTestDatabase(t, "test_solana_indexer_common")
	defer pool.Close()

	notif := "test_notification"
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()

	notifChan := make(chan *pgconn.Notification, 1)

	// Callback to capture the notification
	callback := func(ctx context.Context, notification *pgconn.Notification) {
		notifChan <- notification
	}

	logger := zap.NewNop()
	err := WatchPgNotification(ctx, pool, notif, callback, logger)
	require.NoError(t, err, "failed to listen for notifications")

	conn, err := pool.Acquire(ctx)
	require.NoError(t, err, "failed to acquire database connection")
	defer conn.Release()

	// Send a test notification
	_, err = conn.Exec(ctx, "NOTIFY "+notif+", 'payload'")
	require.NoError(t, err, "failed to send notification")

	// Wait for the notification to be received
	select {
	case <-ctx.Done():
		t.Fatal("timed out waiting for notification")
	case n := <-notifChan:
		require.NotNil(t, n, "notification should not be nil")
		assert.Equal(t, notif, n.Channel, "notification channel should match")
		assert.Equal(t, "payload", n.Payload, "notification payload should match")
	default:
	}

}
