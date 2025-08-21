package comms

import (
	"context"
	"testing"
	"time"

	"bridgerton.audius.co/api/dbv1"
	"bridgerton.audius.co/config"
	"bridgerton.audius.co/trashid"
	"github.com/jackc/pgx/v5"
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
func SetupChatWithMembers(t *testing.T, tx pgx.Tx, ctx context.Context, chatId string, user1Id, user2Id int32, inviteCode1, inviteCode2 string) {
	ts := time.Now()

	// Create chat
	err := chatCreate(tx, ctx, user1Id, ts, ChatCreateRPCParams{
		ChatID: chatId,
		Invites: []PurpleInvite{
			{UserID: trashid.MustEncodeHashID(int(user1Id)), InviteCode: inviteCode1},
			{UserID: trashid.MustEncodeHashID(int(user2Id)), InviteCode: inviteCode2},
		},
	})
	require.NoError(t, err)
}
