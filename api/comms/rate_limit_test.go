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
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TODO: This has been skipped forever, fix it or remove it
func TestRateLimit(t *testing.T) {
	// todo: update for no-nats
	t.Skip()

	// Setup
	pool := database.CreateTestDatabase(t, "test_api")
	defer pool.Close()

	ctx := context.Background()

	// Create validator for validation testing
	validator := CreateTestValidator(t, pool)

	// reset tables under test
	_, err := pool.Exec(ctx, "truncate table chat cascade")
	require.NoError(t, err)

	// Add test rules
	// testRules := map[string]int{
	// 	config.RateLimitTimeframeHours:             24,
	// 	config.RateLimitMaxNumMessages:             3,
	// 	config.RateLimitMaxNumMessagesPerRecipient: 2,
	// 	config.RateLimitMaxNumNewChats:             2,
	// }
	// for rule, limit := range testRules {
	// 	_, err := kv.PutString(rule, strconv.Itoa(limit))
	// 	assert.NoError(t, err)
	// }

	seededRand := rand.New(rand.NewSource(time.Now().UnixNano()))
	user1Id := seededRand.Int31()
	user2Id := seededRand.Int31()
	user3Id := seededRand.Int31()
	user4Id := seededRand.Int31()
	user5Id := seededRand.Int31()

	user1IdEncoded := trashid.MustEncodeHashID(int(user1Id))
	user3IdEncoded := trashid.MustEncodeHashID(int(user3Id))
	user4IdEncoded := trashid.MustEncodeHashID(int(user4Id))
	user5IdEncoded := trashid.MustEncodeHashID(int(user5Id))

	// user1Id created a new chat with user2Id 48 hours ago
	chatId1 := strconv.Itoa(seededRand.Int())
	chatTs := time.Now().UTC().Add(-time.Hour * time.Duration(48))
	_, err = pool.Exec(ctx, "insert into chat (chat_id, created_at, last_message_at) values ($1, $2, $2)", chatId1, chatTs)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, "insert into chat_member (chat_id, invited_by_user_id, invite_code, user_id, created_at) values ($1, $2, $1, $2, $4), ($1, $2, $1, $3, $4)", chatId1, user1Id, user2Id, chatTs)
	require.NoError(t, err)

	// user1Id messaged user2Id 48 hours ago
	err = chatSendMessage(pool, ctx, user1Id, chatId1, "1", chatTs, "Hello")
	require.NoError(t, err)

	// user1Id messages user2Id twice now
	message := "Hello today 1"
	messageRpc := RawRPC{
		Params: []byte(fmt.Sprintf(`{"chat_id": "%s", "message": "%s"}`, chatId1, message)),
	}
	err = validator.validateChatMessage(ctx, user1Id, messageRpc)
	assert.NoError(t, err)
	err = chatSendMessage(pool, ctx, user1Id, chatId1, "2", time.Now().UTC(), message)
	require.NoError(t, err)

	message = "Hello today 2"
	messageRpc = RawRPC{
		Params: []byte(fmt.Sprintf(`{"chat_id": "%s", "message": "%s"}`, chatId1, message)),
	}
	err = validator.validateChatMessage(ctx, user1Id, messageRpc)
	assert.NoError(t, err)

	err = chatSendMessage(pool, ctx, user1Id, chatId1, "3", time.Now().UTC(), message)
	require.NoError(t, err)

	// user1Id messages user2Id a 3rd time
	// Blocked by rate limiter (hit max # messages per recipient in the past 24 hours)
	message = "Hello again again."
	messageRpc = RawRPC{
		Params: []byte(fmt.Sprintf(`{"chat_id": "%s", "message": "%s"}`, chatId1, message)),
	}
	err = validator.validateChatMessage(ctx, user1Id, messageRpc)
	assert.ErrorContains(t, err, "User has exceeded the maximum number of new messages")

	// user1Id creates a new chat with user3Id (1 chat created in 24h)
	chatId2 := strconv.Itoa(seededRand.Int())
	createRpc := RawRPC{
		Params: []byte(fmt.Sprintf(`{"chat_id": "%s", "invites": [{"user_id": "%s", "invite_code": "%s"}, {"user_id": "%s", "invite_code": "%s"}]}`, chatId2, user1IdEncoded, chatId2, user3IdEncoded, chatId2)),
	}
	err = validator.validateChatCreate(ctx, user1Id, createRpc)
	assert.NoError(t, err)

	SetupChatWithMembers(t, pool, ctx, chatId2, user1Id, user3Id, chatId2, chatId2)

	// user1Id messages user3Id
	// Still blocked by rate limiter (hit max # messages with user2Id in the past 24h)
	message = "Hi user3Id"
	messageRpc = RawRPC{
		Params: []byte(fmt.Sprintf(`{"chat_id": "%s", "message": "%s"}`, chatId2, message)),
	}
	err = validator.validateChatMessage(ctx, user1Id, messageRpc)
	assert.ErrorContains(t, err, "User has exceeded the maximum number of new messages")

	// Remove message 3 from db so can test other rate limits
	_, err = pool.Exec(ctx, "delete from chat_message where message_id = '3'")
	require.NoError(t, err)

	// user1Id should be able to message user3Id now
	err = validator.validateChatMessage(ctx, user1Id, messageRpc)
	assert.NoError(t, err)
	err = chatSendMessage(pool, ctx, user1Id, chatId2, "3", time.Now().UTC(), message)
	require.NoError(t, err)

	// user1Id creates a new chat with user4Id (2 chats created in 24h)
	chatId3 := strconv.Itoa(seededRand.Int())
	createRpc = RawRPC{
		Params: []byte(fmt.Sprintf(`{"chat_id": "%s", "invites": [{"user_id": "%s", "invite_code": "%s"}, {"user_id": "%s", "invite_code": "%s"}]}`, chatId3, user1IdEncoded, chatId3, user4IdEncoded, chatId3)),
	}
	err = validator.validateChatCreate(ctx, user1Id, createRpc)
	assert.NoError(t, err)
	SetupChatWithMembers(t, pool, ctx, chatId3, user1Id, user4Id, chatId3, chatId3)

	// user1Id messages user4Id
	message = "Hi user4Id again"
	messageRpc = RawRPC{
		Params: []byte(fmt.Sprintf(`{"chat_id": "%s", "message": "%s"}`, chatId3, message)),
	}
	err = validator.validateChatMessage(ctx, user1Id, messageRpc)
	assert.NoError(t, err)
	err = chatSendMessage(pool, ctx, user1Id, chatId3, "4", time.Now().UTC(), message)
	require.NoError(t, err)

	// user1Id messages user4Id again (4th message to anyone in 24h)
	// Blocked by rate limiter (hit max # messages in the past 24 hours)
	message = "Hi user4Id again"
	messageRpc = RawRPC{
		Params: []byte(fmt.Sprintf(`{"chat_id": "%s", "message": "%s"}`, chatId3, message)),
	}
	err = validator.validateChatMessage(ctx, user1Id, messageRpc)
	assert.ErrorContains(t, err, "User has exceeded the maximum number of new messages")

	// user1Id creates a new chat with user5Id (3 chats created in 24h)
	// Blocked by rate limiter (hit max # new chats)
	chatId4 := strconv.Itoa(seededRand.Int())
	createRpc = RawRPC{
		Params: []byte(fmt.Sprintf(`{"chat_id": "%s", "invites": [{"user_id": "%s", "invite_code": "%s"}, {"user_id": "%s", "invite_code": "%s"}]}`, chatId4, user1IdEncoded, chatId2, user5IdEncoded, chatId4)),
	}
	err = validator.validateChatCreate(ctx, user1Id, createRpc)
	assert.ErrorContains(t, err, "An invited user has exceeded the maximum number of new chats")
}

func TestBurstRateLimit(t *testing.T) {
	// Setup
	pool := database.CreateTestDatabase(t, "test_api")
	defer pool.Close()

	ctx := context.Background()

	// reset tables under test
	_, err := pool.Exec(ctx, "truncate table chat cascade")
	require.NoError(t, err)

	chatId := trashid.ChatID(1, 2) // Use deterministic chat ID
	user1Id := int32(1)
	user2Id := int32(2)

	SetupChatWithMembers(t, pool, ctx, chatId, user1Id, user2Id, chatId, chatId)

	// Create validator for validation testing
	validator := CreateTestValidator(t, pool)

	// hit the 1 second limit... send a burst of messages
	for i := 1; i < 16; i++ {
		message := fmt.Sprintf("burst %d", i)
		err = chatSendMessage(pool, ctx, user1Id, chatId, message, time.Now().UTC(), message)
		assert.NoError(t, err, "i is", i)

		messageRpc := RawRPC{
			Params: []byte(fmt.Sprintf(`{"chat_id": "%s", "message": "%s"}`, chatId, message)),
		}
		err = validator.validateChatMessage(ctx, user1Id, messageRpc)

		// first 10 messages are ok...
		// and then the per-second rate limiter kicks in
		if i <= 10 {
			assert.NoError(t, err, "i is", i)
		} else {
			assert.ErrorIs(t, err, ErrMessageRateLimitExceeded, "i = ", i)
		}
	}
}
