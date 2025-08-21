package comms

import (
	"context"
	"fmt"
	"math/rand"
	"strconv"
	"testing"
	"time"

	"bridgerton.audius.co/database"
	"bridgerton.audius.co/trashid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestChatDeletion(t *testing.T) {
	// Create test database
	pool := database.CreateTestDatabase(t, "test_api")
	defer pool.Close()

	ctx := context.Background()

	// Create deterministic chat ID using trashid.ChatID
	chatId := trashid.ChatID(1, 2)

	// Start transaction
	tx, err := pool.Begin(ctx)
	require.NoError(t, err)
	defer tx.Rollback(ctx)

	// Setup chat with members
	seededRand := rand.New(rand.NewSource(time.Now().UnixNano()))
	inviteCode1 := strconv.Itoa(seededRand.Int())
	inviteCode2 := strconv.Itoa(seededRand.Int())

	SetupChatWithMembers(t, tx, ctx, chatId, 1, 2, inviteCode1, inviteCode2)

	// Commit the transaction so the validator can see the data
	err = tx.Commit(ctx)
	require.NoError(t, err)

	assertDeleted := func(chatId string, userId int32, expectDeleted bool) {
		row := pool.QueryRow(ctx, "select cleared_history_at from chat_member where chat_id = $1 and user_id = $2", chatId, userId)
		var clearedHistoryAt pgtype.Timestamp
		err = row.Scan(&clearedHistoryAt)
		assert.NoError(t, err)
		if expectDeleted {
			assert.True(t, clearedHistoryAt.Valid)
		} else {
			assert.False(t, clearedHistoryAt.Valid)
		}
	}

	// Create a test validator for validation
	validator := CreateTestValidator(t, pool)

	// Test validation using the validator
	{
		// Test that user1Id can delete their chat (they are a member)
		exampleRpc := RawRPC{
			Params: []byte(fmt.Sprintf(`{"chat_id": "%s"}`, chatId)),
		}

		err = validator.validateChatDelete(1, exampleRpc)
		assert.NoError(t, err, "User 1 should be able to delete their chat")

		// Test that user3Id cannot delete the chat (they are not a member)
		err = validator.validateChatDelete(3, exampleRpc)
		assert.Error(t, err, "User 3 should not be able to delete the chat")
		assert.Contains(t, err.Error(), "user is not a member of this chat")
	}

	// user1Id deletes the chat
	deleteTs := time.Now()
	tx2, err := pool.Begin(ctx)
	require.NoError(t, err)
	defer tx2.Rollback(ctx)

	err = chatDelete(tx2, ctx, 1, chatId, deleteTs)
	assert.NoError(t, err)

	err = tx2.Commit(ctx)
	assert.NoError(t, err)

	assertDeleted(chatId, 1, true)

	// chat is not deleted for user2Id
	assertDeleted(chatId, 2, false)
}
