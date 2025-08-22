package comms

import (
	"context"
	"testing"
	"time"

	"bridgerton.audius.co/api/dbv1"
	"bridgerton.audius.co/config"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// CreateTestValidator creates a validator instance for testing
func CreateTestValidator(t *testing.T, pool *pgxpool.Pool) *Validator {
	limiter, err := NewRateLimiter()
	require.NoError(t, err)

	// Create a minimal test config
	testConfig := &config.Config{
		Env:              "test",
		AntiAbuseOracles: []string{}, // Empty for tests
	}

	// Create a test logger
	logger := zap.NewNop()

	// Create DBPools for the validator
	dbPools := &dbv1.DBPools{
		Replicas: []*pgxpool.Pool{pool},
	}

	// Create validator
	return NewValidator(dbPools, limiter, testConfig, logger)
}

// SetupChatWithMembers creates a chat with the given members for testing
func SetupChatWithMembers(t *testing.T, db dbv1.DBTX, ctx context.Context, chatId string, user1Id, user2Id int32, inviteCode1, inviteCode2 string) {
	ts := time.Now().UTC()

	// Create chat record
	_, err := db.Exec(ctx, "INSERT INTO chat (chat_id, created_at, last_message_at) VALUES ($1, $2, $2)", chatId, ts)
	require.NoError(t, err)

	// Insert two members in a single statement
	_, err = db.Exec(ctx, "INSERT INTO chat_member (chat_id, invited_by_user_id, invite_code, user_id, created_at) VALUES ($1, $2, $3, $2, $5), ($1, $2, $4, $6, $5)", chatId, user1Id, inviteCode1, inviteCode2, ts, user2Id)
	require.NoError(t, err)
}
