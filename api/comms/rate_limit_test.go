package comms

import (
	"context"
	"fmt"
	"testing"
	"time"

	"bridgerton.audius.co/database"
	"bridgerton.audius.co/trashid"
	"github.com/stretchr/testify/assert"
)

func TestBurstRateLimit(t *testing.T) {
	// Setup
	pool := database.CreateTestDatabase(t, "test_comms")
	defer pool.Close()

	ctx := context.Background()

	chatId := trashid.ChatID(1, 2) // Use deterministic chat ID
	user1Id := int32(1)
	user2Id := int32(2)

	SetupChatWithMembers(t, pool, ctx, chatId, user1Id, user2Id, chatId, chatId)

	rateLimitConfig := DefaultRateLimitConfig
	rateLimitConfig.MaxMessagesPerRecipient1s = 2

	// Create validator for validation testing
	validator := CreateTestValidator(t, pool, rateLimitConfig, DefaultTestValidatorConfig)

	// hit the 1 second limit... send a burst of messages
	for i := 1; i < 5; i++ {
		message := fmt.Sprintf("burst %d", i)
		err := chatSendMessage(pool, ctx, user1Id, chatId, message, time.Now().UTC(), message)
		assert.NoError(t, err, "i is", i)

		messageRpc := RawRPC{
			Params: []byte(fmt.Sprintf(`{"chat_id": "%s", "message": "%s"}`, chatId, message)),
		}
		err = validator.validateChatMessage(ctx, user1Id, messageRpc)

		// first 2 messages are ok...
		// and then the per-second rate limiter kicks in
		if i <= 2 {
			assert.NoError(t, err, "i is", i)
		} else {
			assert.ErrorIs(t, err, ErrMessageRateLimitExceeded, "i = ", i, err)
		}
	}
}
