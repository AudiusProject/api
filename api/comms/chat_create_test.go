package comms

import (
	"context"
	"testing"
	"time"

	"bridgerton.audius.co/database"
	"bridgerton.audius.co/trashid"
	"github.com/stretchr/testify/assert"
)

func TestChatCreate(t *testing.T) {
	// Create test database
	pool := database.CreateTestDatabase(t, "test_api")
	defer pool.Close()

	ctx := context.Background()

	// Create deterministic chat ID using trashid.ChatID
	chatId := trashid.ChatID(1, 2)

	var count int

	tsEarly := time.Now().Add(-time.Minute)
	tsLate := time.Now().Add(-time.Second)
	tsLater := time.Now()

	// Create a chat with a later timestamp
	err := chatCreate(pool, ctx, 1, tsLate, ChatCreateRPCParams{
		ChatID: chatId,
		Invites: []PurpleInvite{
			{UserID: trashid.MustEncodeHashID(1), InviteCode: "later"},
			{UserID: trashid.MustEncodeHashID(2), InviteCode: "later"},
		},
	})
	assert.NoError(t, err)

	// Send a message in this errant chat
	err = chatSendMessage(pool, ctx, 1, chatId, "bad_message", tsLate, "this message is doomed")
	assert.NoError(t, err)

	err = pool.QueryRow(ctx, `select count(*) from chat_message where chat_id = $1`, chatId).Scan(&count)
	assert.NoError(t, err)
	assert.Equal(t, 1, count)

	// Now create a "delayed" chat... that was timestamped earlier, but arrived later
	err = chatCreate(pool, ctx, 1, tsEarly, ChatCreateRPCParams{
		ChatID: chatId,
		Invites: []PurpleInvite{
			{UserID: trashid.MustEncodeHashID(1), InviteCode: "earlier"},
			{UserID: trashid.MustEncodeHashID(2), InviteCode: "earlier"},
		},
	})
	assert.NoError(t, err)

	// Now create a "delayed" chat... that was timestamped later and arrives later
	err = chatCreate(pool, ctx, 1, tsLater, ChatCreateRPCParams{
		ChatID: chatId,
		Invites: []PurpleInvite{
			{UserID: trashid.MustEncodeHashID(1), InviteCode: "even_later"},
			{UserID: trashid.MustEncodeHashID(2), InviteCode: "even_later"},
		},
	})
	assert.NoError(t, err)

	// Send a message in this earlier chat
	err = chatSendMessage(pool, ctx, 1, chatId, "good_message", tsLate, "this message is blessed")
	assert.NoError(t, err)

	err = pool.QueryRow(ctx, `select count(*) from chat_message where chat_id = $1`, chatId).Scan(&count)
	assert.NoError(t, err)
	assert.Equal(t, 2, count)

	// Verify that the "earlier" invite codes are the ones that persisted
	err = pool.QueryRow(ctx, `select count(*) from chat_member where invite_code = 'earlier'`).Scan(&count)
	assert.NoError(t, err)
	assert.Equal(t, 2, count)

	// Verify that the "later" invite codes were overwritten
	err = pool.QueryRow(ctx, `select count(*) from chat_member where invite_code = 'later'`).Scan(&count)
	assert.NoError(t, err)
	assert.Equal(t, 0, count)

	// Verify that the "even_later" invite codes were not used
	err = pool.QueryRow(ctx, `select count(*) from chat_member where invite_code = 'even_later'`).Scan(&count)
	assert.NoError(t, err)
	assert.Equal(t, 0, count)
}
