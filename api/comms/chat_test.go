package comms

import (
	"context"
	"fmt"
	"math/rand"
	"strconv"
	"testing"
	"time"

	"api.audius.co/database"
	"api.audius.co/trashid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
)

func TestChat(t *testing.T) {
	pool := database.CreateTestDatabase(t, "test_comms")
	defer pool.Close()

	ctx := context.Background()

	seededRand := rand.New(rand.NewSource(time.Now().UnixNano()))
	chatId := trashid.ChatID(1, 2) // Use deterministic chat ID
	user1Id := int32(1)
	user2Id := int32(2)
	user3Id := int32(3)

	SetupChatWithMembers(t, pool, ctx, chatId, user1Id, user2Id, "test1", "test2")

	validator := CreateTestValidator(t, pool, DefaultRateLimitConfig, DefaultTestValidatorConfig)

	// validate user1Id and user2Id can both send messages in this chat
	{
		exampleRpc := RawRPC{
			Params: []byte(fmt.Sprintf(`{"chat_id": "%s", "message": "test123"}`, chatId)),
		}

		err := validator.validateChatMessage(ctx, user1Id, exampleRpc)
		assert.NoError(t, err)

		err = validator.validateChatMessage(ctx, user3Id, exampleRpc)
		assert.Contains(t, err.Error(), "user is not a member of this chat")
	}

	// user1Id sends user2Id a message
	messageTs := time.Now()
	messageId := strconv.Itoa(seededRand.Int())
	err := chatSendMessage(pool, ctx, user1Id, chatId, messageId, messageTs, "hello user2Id")
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

	// assert sender has no unread messages
	assertUnreadCount(chatId, user1Id, 0)

	// assert user2Id has one unread message
	assertUnreadCount(chatId, user2Id, 1)

	// user2Id reads message
	err = chatReadMessages(pool, ctx, user2Id, chatId, time.Now())
	assert.NoError(t, err)

	// assert user2Id has zero unread messages
	assertUnreadCount(chatId, user2Id, 0)

	// user2Id sends a reply to user1Id
	replyTs := time.Now()
	replyMessageId := "2"
	err = chatSendMessage(pool, ctx, user2Id, chatId, replyMessageId, replyTs, "oh hey there user1 thanks for your message")
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

	// user1Id reacts to user2Id's message
	reactTs := time.Now().Add(-time.Second)
	reaction := "fire"
	err = chatReactMessage(pool, ctx, user1Id, chatId, replyMessageId, &reaction, reactTs)
	assert.NoError(t, err)

	assertReaction(user1Id, replyMessageId, &reaction)

	// user1Id changes reaction to user2Id's message
	changedReactTs := time.Now()
	newReaction := "heart"
	err = chatReactMessage(pool, ctx, user1Id, chatId, replyMessageId, &newReaction, changedReactTs)
	assert.NoError(t, err)

	assertReaction(user1Id, replyMessageId, &newReaction)

	// if an "older" reaction arrives late... it will not overwrite newer reaction
	err = chatReactMessage(pool, ctx, user1Id, chatId, replyMessageId, &reaction, reactTs)
	assert.NoError(t, err)

	assertReaction(user1Id, replyMessageId, &newReaction)

	// if an "older" delete arrives late... it is ignored
	err = chatReactMessage(pool, ctx, user1Id, chatId, replyMessageId, nil, reactTs)
	assert.NoError(t, err)

	assertReaction(user1Id, replyMessageId, &newReaction)

	// user1Id removes reaction to user2Id's message
	removedReactTs := time.Now()
	err = chatReactMessage(pool, ctx, user1Id, chatId, replyMessageId, nil, removedReactTs)
	assert.NoError(t, err)

	assertReaction(user1Id, replyMessageId, nil)
}

func TestChatReadAllMessages(t *testing.T) {
	pool := database.CreateTestDatabase(t, "test_comms_read_all")
	defer pool.Close()

	ctx := context.Background()
	seededRand := rand.New(rand.NewSource(time.Now().UnixNano()))

	user1Id := int32(1)
	user2Id := int32(2)
	user3Id := int32(3)

	chatA := trashid.ChatID(int(user1Id), int(user2Id))
	chatB := trashid.ChatID(int(user1Id), int(user3Id))
	SetupChatWithMembers(t, pool, ctx, chatA, user1Id, user2Id, "a1", "a2")
	SetupChatWithMembers(t, pool, ctx, chatB, user1Id, user3Id, "b1", "b3")

	assertUnreadCount := func(chatId string, userId int32, expected int) {
		t.Helper()
		unreadCount := 0
		err := pool.QueryRow(ctx, "select unread_count from chat_member where chat_id = $1 and user_id = $2", chatId, userId).Scan(&unreadCount)
		assert.NoError(t, err)
		assert.Equal(t, expected, unreadCount, "unread for chat %s user %d", chatId, userId)
	}

	// Send user1Id one message in each chat from the other party.
	err := chatSendMessage(pool, ctx, user2Id, chatA, strconv.Itoa(seededRand.Int()), time.Now(), "hi from 2")
	assert.NoError(t, err)
	err = chatSendMessage(pool, ctx, user3Id, chatB, strconv.Itoa(seededRand.Int()), time.Now(), "hi from 3")
	assert.NoError(t, err)

	assertUnreadCount(chatA, user1Id, 1)
	assertUnreadCount(chatB, user1Id, 1)
	// Senders' own unread counts stay at zero.
	assertUnreadCount(chatA, user2Id, 0)
	assertUnreadCount(chatB, user3Id, 0)

	// Single call clears every unread chat for user1Id without touching
	// the other members' chats.
	readTs := time.Now()
	err = chatReadAllMessages(pool, ctx, user1Id, readTs)
	assert.NoError(t, err)

	assertUnreadCount(chatA, user1Id, 0)
	assertUnreadCount(chatB, user1Id, 0)

	// Re-confirm: a stale read (older timestamp) does NOT roll back.
	// Add a new unread, advance via chatReadAllMessages, then try a stale read.
	err = chatSendMessage(pool, ctx, user2Id, chatA, strconv.Itoa(seededRand.Int()), time.Now(), "another from 2")
	assert.NoError(t, err)
	assertUnreadCount(chatA, user1Id, 1)

	freshTs := time.Now()
	err = chatReadAllMessages(pool, ctx, user1Id, freshTs)
	assert.NoError(t, err)
	assertUnreadCount(chatA, user1Id, 0)

	// Older timestamp must be a no-op for last_active_at.
	err = chatReadAllMessages(pool, ctx, user1Id, freshTs.Add(-time.Hour))
	assert.NoError(t, err)
	var lastActive time.Time
	err = pool.QueryRow(ctx, "select last_active_at from chat_member where chat_id = $1 and user_id = $2", chatA, user1Id).Scan(&lastActive)
	assert.NoError(t, err)
	assert.WithinDuration(t, freshTs.UTC(), lastActive.UTC(), time.Second, "stale read should not roll back last_active_at")
}
