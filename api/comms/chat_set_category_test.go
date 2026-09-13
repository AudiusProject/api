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
	"github.com/stretchr/testify/require"
)

func TestChatSetCategory(t *testing.T) {
	pool := database.CreateTestDatabase(t, "test_comms")
	defer pool.Close()

	ctx := context.Background()

	chatId := trashid.ChatID(1, 2)

	seededRand := rand.New(rand.NewSource(time.Now().UnixNano()))
	inviteCode1 := strconv.Itoa(seededRand.Int())
	inviteCode2 := strconv.Itoa(seededRand.Int())

	SetupChatWithMembers(t, pool, ctx, chatId, 1, 2, inviteCode1, inviteCode2)

	// getCategory returns the stored category for (userId, chatId), or nil when
	// there is no row (uncategorized).
	getCategory := func(userId int32) *string {
		var category string
		err := pool.QueryRow(ctx,
			"select category from user_conversation_preferences where user_id = $1 and chat_id = $2",
			userId, chatId).Scan(&category)
		if err == pgx.ErrNoRows {
			return nil
		}
		require.NoError(t, err)
		return &category
	}

	validator := CreateTestValidator(t, pool, DefaultRateLimitConfig, DefaultTestValidatorConfig)

	// Validation
	{
		priorityRpc := RawRPC{
			Params: []byte(fmt.Sprintf(`{"chat_id": "%s", "category": "priority"}`, chatId)),
		}
		generalRpc := RawRPC{
			Params: []byte(fmt.Sprintf(`{"chat_id": "%s", "category": "general"}`, chatId)),
		}
		nullRpc := RawRPC{
			Params: []byte(fmt.Sprintf(`{"chat_id": "%s", "category": null}`, chatId)),
		}
		bogusRpc := RawRPC{
			Params: []byte(fmt.Sprintf(`{"chat_id": "%s", "category": "spam"}`, chatId)),
		}

		// members may set any valid category or clear it
		assert.NoError(t, validator.validateChatSetCategory(1, priorityRpc))
		assert.NoError(t, validator.validateChatSetCategory(2, generalRpc))
		assert.NoError(t, validator.validateChatSetCategory(1, nullRpc))

		// non-members are rejected
		err := validator.validateChatSetCategory(3, priorityRpc)
		assert.Error(t, err, "User 3 is not a member and should not be able to set a category")
		assert.Contains(t, err.Error(), "user is not a member of this chat")

		// unknown category values are rejected, even for members
		err = validator.validateChatSetCategory(1, bogusRpc)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid chat category")

		// the full Validate entrypoint routes chat.set_category to the validator
		err = validator.Validate(ctx, 3, RawRPC{Method: string(RPCMethodChatSetCategory), Params: priorityRpc.Params})
		assert.Error(t, err)
	}

	general := string(ChatCategoryGeneral)
	priority := string(ChatCategoryPriority)

	// no row to start with
	assert.Nil(t, getCategory(1))

	// set "general"
	t1 := time.Now().UTC().Add(-time.Minute)
	require.NoError(t, chatSetCategory(pool, ctx, 1, chatId, &general, t1))
	if got := getCategory(1); assert.NotNil(t, got) {
		assert.Equal(t, general, *got)
	}

	// update to "priority" with a newer timestamp
	t2 := t1.Add(10 * time.Second)
	require.NoError(t, chatSetCategory(pool, ctx, 1, chatId, &priority, t2))
	if got := getCategory(1); assert.NotNil(t, got) {
		assert.Equal(t, priority, *got)
	}

	// a late-arriving older RPC must not clobber newer state
	tOld := t1.Add(5 * time.Second)
	require.NoError(t, chatSetCategory(pool, ctx, 1, chatId, &general, tOld))
	if got := getCategory(1); assert.NotNil(t, got) {
		assert.Equal(t, priority, *got, "older set_category should be ignored")
	}

	// the other member's preference is independent
	assert.Nil(t, getCategory(2))

	// clearing with an older timestamp must not delete the newer row
	require.NoError(t, chatSetCategory(pool, ctx, 1, chatId, nil, tOld))
	if got := getCategory(1); assert.NotNil(t, got) {
		assert.Equal(t, priority, *got, "older clear should be ignored")
	}

	// clearing with a newer timestamp deletes the row
	t3 := t2.Add(10 * time.Second)
	require.NoError(t, chatSetCategory(pool, ctx, 1, chatId, nil, t3))
	assert.Nil(t, getCategory(1))

	// clearing when there is no row is a no-op, not an error
	require.NoError(t, chatSetCategory(pool, ctx, 1, chatId, nil, t3.Add(time.Second)))
	assert.Nil(t, getCategory(1))
}
