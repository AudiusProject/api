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
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestChat(t *testing.T) {
	// Create test database
	pool := database.CreateTestDatabase(t, "test_api")
	defer pool.Close()

	ctx := context.Background()

	// Start transaction
	tx, err := pool.Begin(ctx)
	require.NoError(t, err)
	defer tx.Rollback(ctx)

	seededRand := rand.New(rand.NewSource(time.Now().UnixNano()))
	chatId := trashid.ChatID(1, 2) // Use deterministic chat ID
	user1Id := int32(1)
	user2Id := int32(2)
	user3Id := int32(3)

	SetupChatWithMembers(t, tx, ctx, chatId, user1Id, user2Id, "test1", "test2")

	// Commit the transaction so the validator can see the data
	err = tx.Commit(ctx)
	require.NoError(t, err)

	// Create validator for validation testing
	validator := CreateTestValidator(t, pool)

	// validate user1Id and user2Id can both send messages in this chat
	{
		exampleRpc := RawRPC{
			Params: []byte(fmt.Sprintf(`{"chat_id": "%s", "message": "test123"}`, chatId)),
		}

		err = validator.validateChatMessage(ctx, user1Id, exampleRpc)
		assert.NoError(t, err)

		err = validator.validateChatMessage(ctx, user3Id, exampleRpc)
		assert.Contains(t, err.Error(), "user is not a member of this chat")
	}

	// Start a new transaction for the rest of the test
	tx2, err := pool.Begin(ctx)
	require.NoError(t, err)
	defer tx2.Rollback(ctx)

	// user1Id sends user2Id a message
	messageTs := time.Now()
	messageId := strconv.Itoa(seededRand.Int())
	err = chatSendMessage(tx2, ctx, user1Id, chatId, messageId, messageTs, "hello user2Id")
	assert.NoError(t, err)

	// assertUnreadCount helper fun in a closure
	assertUnreadCount := func(chatId string, userId int32, expected int) {
		unreadCount := 0
		err := pool.QueryRow(ctx, "select unread_count from chat_member where chat_id = $1 and user_id = $2", chatId, userId).Scan(&unreadCount)
		assert.NoError(t, err)
		assert.Equal(t, expected, unreadCount)
	}

	assertReaction := func(userId int32, messageId string, expected *string) {
		var reaction string
		err := pool.QueryRow(ctx, "select reaction from chat_message_reactions where user_id = $1 and message_id = $2", userId, messageId).Scan(&reaction)
		if expected != nil {
			assert.NoError(t, err)
			assert.Equal(t, *expected, reaction)
		} else {
			assert.ErrorIs(t, err, pgx.ErrNoRows)
		}
	}

	// Commit the message transaction
	err = tx2.Commit(ctx)
	assert.NoError(t, err)

	// assert sender has no unread messages
	assertUnreadCount(chatId, user1Id, 0)

	// assert user2Id has one unread message
	assertUnreadCount(chatId, user2Id, 1)

	// Start a new transaction for reading messages
	tx3, err := pool.Begin(ctx)
	require.NoError(t, err)
	defer tx3.Rollback(ctx)

	// user2Id reads message
	err = chatReadMessages(tx3, ctx, user2Id, chatId, time.Now())
	assert.NoError(t, err)

	// Commit the read transaction
	err = tx3.Commit(ctx)
	assert.NoError(t, err)

	// assert user2Id has zero unread messages
	assertUnreadCount(chatId, user2Id, 0)

	// Start a new transaction for the reply
	tx4, err := pool.Begin(ctx)
	require.NoError(t, err)
	defer tx4.Rollback(ctx)

	// user2Id sends a reply to user1Id
	replyTs := time.Now()
	replyMessageId := "2"
	err = chatSendMessage(tx4, ctx, user2Id, chatId, replyMessageId, replyTs, "oh hey there user1 thanks for your message")
	assert.NoError(t, err)

	// Commit the reply transaction
	err = tx4.Commit(ctx)
	assert.NoError(t, err)

	// the tables have turned!
	assertUnreadCount(chatId, user2Id, 0)
	assertUnreadCount(chatId, user1Id, 1)

	// validate user1Id and user2Id can both send reactions in this chat
	{
		exampleRpc := RawRPC{
			Params: []byte(fmt.Sprintf(`{"chat_id": "%s", "message_id": "%s", "reaction": "heart"}`, chatId, replyMessageId)),
		}

		err = validator.validateChatReact(validator.pool, ctx, user1Id, exampleRpc)
		assert.NoError(t, err)

		err = validator.validateChatReact(validator.pool, ctx, user3Id, exampleRpc)
		assert.Contains(t, err.Error(), "user is not a member of this chat")
	}

	// Start a new transaction for reactions
	tx5, err := pool.Begin(ctx)
	require.NoError(t, err)
	defer tx5.Rollback(ctx)

	// user1Id reacts to user2Id's message
	reactTs := time.Now().Add(-time.Second)
	reaction := "fire"
	err = chatReactMessage(tx5, ctx, user1Id, chatId, replyMessageId, &reaction, reactTs)
	assert.NoError(t, err)

	// Commit the reaction transaction
	err = tx5.Commit(ctx)
	assert.NoError(t, err)

	assertReaction(user1Id, replyMessageId, &reaction)

	// Start a new transaction for changing reaction
	tx6, err := pool.Begin(ctx)
	require.NoError(t, err)
	defer tx6.Rollback(ctx)

	// user1Id changes reaction to user2Id's message
	changedReactTs := time.Now()
	newReaction := "heart"
	err = chatReactMessage(tx6, ctx, user1Id, chatId, replyMessageId, &newReaction, changedReactTs)
	assert.NoError(t, err)

	// Commit the change reaction transaction
	err = tx6.Commit(ctx)
	assert.NoError(t, err)

	assertReaction(user1Id, replyMessageId, &newReaction)

	// Start a new transaction for testing older reaction
	tx7, err := pool.Begin(ctx)
	require.NoError(t, err)
	defer tx7.Rollback(ctx)

	// if an "older" reaction arrives late... it will not overwrite newer reaction
	err = chatReactMessage(tx7, ctx, user1Id, chatId, replyMessageId, &reaction, reactTs)
	assert.NoError(t, err)

	// Commit the older reaction transaction
	err = tx7.Commit(ctx)
	assert.NoError(t, err)

	assertReaction(user1Id, replyMessageId, &newReaction)

	// Start a new transaction for testing older delete
	tx8, err := pool.Begin(ctx)
	require.NoError(t, err)
	defer tx8.Rollback(ctx)

	// if an "older" delete arrives late... it is ignored
	err = chatReactMessage(tx8, ctx, user1Id, chatId, replyMessageId, nil, reactTs)
	assert.NoError(t, err)

	// Commit the older delete transaction
	err = tx8.Commit(ctx)
	assert.NoError(t, err)

	assertReaction(user1Id, replyMessageId, &newReaction)

	// Start a new transaction for removing reaction
	tx9, err := pool.Begin(ctx)
	require.NoError(t, err)
	defer tx9.Rollback(ctx)

	// user1Id removes reaction to user2Id's message
	removedReactTs := time.Now()
	err = chatReactMessage(tx9, ctx, user1Id, chatId, replyMessageId, nil, removedReactTs)
	assert.NoError(t, err)

	// Commit the remove reaction transaction
	err = tx9.Commit(ctx)
	assert.NoError(t, err)

	assertReaction(user1Id, replyMessageId, nil)
}
