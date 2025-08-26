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

func TestChatBlocking(t *testing.T) {
	pool := database.CreateTestDatabase(t, "test_comms")
	defer pool.Close()

	ctx := context.Background()

	// Create validator for validation testing
	validator := CreateTestValidator(t, pool, DefaultRateLimitConfig, DefaultTestValidatorConfig)

	// reset tables under test
	_, err := pool.Exec(ctx, "truncate table chat_blocked_users cascade")
	require.NoError(t, err)
	_, err = pool.Exec(ctx, "truncate table chat cascade")
	require.NoError(t, err)

	seededRand := rand.New(rand.NewSource(time.Now().UnixNano()))
	user1Id := seededRand.Int31()
	user2Id := seededRand.Int31()

	assertBlocked := func(blockerUserId int32, blockeeUserId int32, timestamp time.Time, expected int) {
		row := pool.QueryRow(ctx, "select count(*) from chat_blocked_users where blocker_user_id = $1 and blockee_user_id = $2 and created_at = $3", blockerUserId, blockeeUserId, timestamp)
		var count int
		err = row.Scan(&count)
		assert.NoError(t, err)
		assert.Equal(t, expected, count)
	}

	messageTs := time.Now()

	// validate user1Id can block user2Id
	{
		encodedUserId := trashid.MustEncodeHashID(int(user2Id))
		exampleRpc := RawRPC{
			Params: []byte(fmt.Sprintf(`{"user_id": "%s"}`, encodedUserId)),
		}

		err = validator.validateChatBlock(user1Id, exampleRpc)
		assert.NoError(t, err)
	}

	// user1Id blocks user2Id
	{
		err := chatBlock(pool, ctx, user1Id, user2Id, messageTs)
		assert.NoError(t, err)
		assertBlocked(user1Id, user2Id, messageTs, 1)
	}

	// assert no update if duplicate block requests
	{
		duplicateMessageTs := time.Now()
		err := chatBlock(pool, ctx, user1Id, user2Id, duplicateMessageTs)
		assert.NoError(t, err)
		assertBlocked(user1Id, user2Id, messageTs, 1)
		assertBlocked(user1Id, user2Id, duplicateMessageTs, 0)
	}

	// assert a "late" unblock message is ignored
	{
		err := chatBlock(pool, ctx, user1Id, user2Id, time.Now().Add(-time.Hour))
		assert.NoError(t, err)
		assertBlocked(user1Id, user2Id, messageTs, 1)
	}

	// validate user1Id and user2Id cannot create a chat with each other
	{
		chatId := strconv.Itoa(seededRand.Int())
		user1IdEncoded := trashid.MustEncodeHashID(int(user1Id))
		user2IdEncoded := trashid.MustEncodeHashID(int(user2Id))

		exampleRpc := RawRPC{
			Params: []byte(fmt.Sprintf(`{"chat_id": "%s", "invites": [{"user_id": "%s", "invite_code": "1"}, {"user_id": "%s", "invite_code": "2"}]}`, chatId, user1IdEncoded, user2IdEncoded)),
		}

		err := validator.validateChatCreate(ctx, user2Id, exampleRpc)
		assert.ErrorContains(t, err, "Not permitted to send messages to this user")
	}

	// validate user1Id and user2Id cannot message each other
	{
		// Assume chat was already created before blocking
		chatId := strconv.Itoa(seededRand.Int())
		SetupChatWithMembers(t, pool, ctx, chatId, user1Id, user2Id, "invite1", "invite2")

		exampleRpc := RawRPC{
			Params: []byte(fmt.Sprintf(`{"chat_id": "%s", "message_id": "1", "message": "test"}`, chatId)),
		}

		err := validator.validateChatMessage(ctx, user1Id, exampleRpc)
		assert.ErrorContains(t, err, "Not permitted to send messages to this user")
	}

	// user1Id unblocks user2Id
	{
		err := chatUnblock(pool, ctx, user1Id, user2Id, time.Now())
		assert.NoError(t, err)
		assertBlocked(user1Id, user2Id, messageTs, 0)
	}
}
