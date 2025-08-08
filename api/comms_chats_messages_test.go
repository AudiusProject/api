package api

import (
	"fmt"
	"testing"
	"time"

	"bridgerton.audius.co/database"
	"bridgerton.audius.co/trashid"
	"github.com/stretchr/testify/assert"
)

func TestGetChatMessages(t *testing.T) {
	app := emptyTestApp(t)

	// Setup test data
	now := time.Now()
	fixtures := database.FixtureMap{
		"users": {
			{
				"user_id":    1,
				"handle":     "user1",
				"wallet":     "0x7d273271690538cf855e5b3002a0dd8c154bb060",
				"created_at": now.Add(-time.Hour),
				"updated_at": now.Add(-time.Hour),
				"is_current": true,
			},
			{
				"user_id":    2,
				"handle":     "user2",
				"wallet":     "0xc3d1d41e6872ffbd15c473d14fc3a9250be5b5e0",
				"created_at": now.Add(-time.Hour),
				"updated_at": now.Add(-time.Hour),
				"is_current": true,
			},
		},
		"chat": {
			{
				"chat_id":         "test-chat-1",
				"created_at":      now.Add(-time.Hour),
				"last_message_at": now.Add(-time.Minute * 5),
			},
		},
		"chat_member": {
			{
				"chat_id":            "test-chat-1",
				"user_id":            1,
				"invited_by_user_id": 1,
				"invite_code":        "",
				"created_at":         now.Add(-time.Hour),
			},
			{
				"chat_id":            "test-chat-1",
				"user_id":            2,
				"invited_by_user_id": 1,
				"invite_code":        "",
				"created_at":         now.Add(-time.Hour),
			},
		},
		"chat_message": {
			{
				"message_id": "msg1",
				"chat_id":    "test-chat-1",
				"user_id":    1,
				"created_at": now.Add(-time.Minute * 10),
				"ciphertext": "Hello World",
			},
			{
				"message_id": "msg2",
				"chat_id":    "test-chat-1",
				"user_id":    2,
				"created_at": now.Add(-time.Minute * 8),
				"ciphertext": "Hi there!",
			},
			{
				"message_id": "msg3",
				"chat_id":    "test-chat-1",
				"user_id":    1,
				"created_at": now.Add(-time.Minute * 5),
				"ciphertext": "How are you?",
			},
		},
		"chat_blast": {
			{
				"blast_id":     "blast1",
				"from_user_id": 1,
				"audience":     "follower_audience",
				"plaintext":    "Hello followers!",
				"created_at":   now.Add(-time.Minute * 15),
			},
			{
				"blast_id":              "blast2",
				"from_user_id":          1,
				"audience":              "customer_audience",
				"audience_content_type": "track",
				"audience_content_id":   123,
				"plaintext":             "Thanks for buying my track!",
				"created_at":            now.Add(-time.Minute * 12),
			},
		},
	}

	database.Seed(app.pool, fixtures)

	t.Run("get regular chat messages", func(t *testing.T) {
		// Test getting regular chat messages (not blasts)
		status, body := testGetWithWallet(t, app, "/comms/chats/test-chat-1/messages", "0x7d273271690538cf855e5b3002a0dd8c154bb060")
		assert.Equal(t, 200, status)

		jsonAssert(t, body, map[string]any{
			"data.0.message_id":     "msg3",
			"data.0.sender_user_id": trashid.MustEncodeHashID(1),
			"data.0.message":        "How are you?",
			"data.0.is_plaintext":   false,
			"data.1.message_id":     "msg2",
			"data.1.sender_user_id": trashid.MustEncodeHashID(2),
			"data.1.message":        "Hi there!",
			"data.2.message_id":     "msg1",
			"data.2.sender_user_id": trashid.MustEncodeHashID(1),
			"data.2.message":        "Hello World",
			"health.is_healthy":     true,
		})
	})

	t.Run("get chat messages with limit", func(t *testing.T) {
		status, body := testGetWithWallet(t, app, "/comms/chats/test-chat-1/messages?limit=2", "0x7d273271690538cf855e5b3002a0dd8c154bb060")
		assert.Equal(t, 200, status)

		jsonAssert(t, body, map[string]any{
			"data.0.message_id": "msg3",
			"data.1.message_id": "msg2",
			"data.2":            nil, // Should only have 2 messages due to limit
		})
	})

	t.Run("get blast messages - follower audience", func(t *testing.T) {
		status, body := testGetWithWallet(t, app, "/comms/chats/follower_audience/messages?is_blast=true", "0x7d273271690538cf855e5b3002a0dd8c154bb060")
		assert.Equal(t, 200, status)

		jsonAssert(t, body, map[string]any{
			"data.0.message_id":     "blast1",
			"data.0.sender_user_id": trashid.MustEncodeHashID(1),
			"data.0.message":        "Hello followers!",
			"data.0.is_plaintext":   true,
			"health.is_healthy":     true,
		})
	})

	t.Run("get blast messages - customer audience with content", func(t *testing.T) {
		chatID := fmt.Sprintf("customer_audience:track:%s", trashid.MustEncodeHashID(123))
		url := fmt.Sprintf("/comms/chats/%s/messages?is_blast=true", chatID)

		status, body := testGetWithWallet(t, app, url, "0x7d273271690538cf855e5b3002a0dd8c154bb060")
		assert.Equal(t, 200, status)

		jsonAssert(t, body, map[string]any{
			"data.0.message_id":     "blast2",
			"data.0.sender_user_id": trashid.MustEncodeHashID(1),
			"data.0.message":        "Thanks for buying my track!",
			"data.0.is_plaintext":   true,
			"health.is_healthy":     true,
		})
	})

	t.Run("invalid blast audience", func(t *testing.T) {
		status, _ := testGetWithWallet(t, app, "/comms/chats/invalid_audience/messages?is_blast=true", "0x7d273271690538cf855e5b3002a0dd8c154bb060")
		assert.Equal(t, 500, status) // Should return error for unsupported audience
	})

	t.Run("invalid chat id for blast", func(t *testing.T) {
		status, _ := testGetWithWallet(t, app, "/comms/chats//messages?is_blast=true", "0x7d273271690538cf855e5b3002a0dd8c154bb060")
		assert.Equal(t, 503, status) // Returns 503 because empty path doesn't match the route
	})

	t.Run("no messages found", func(t *testing.T) {
		// Test with non-existent chat
		status, body := testGetWithWallet(t, app, "/comms/chats/nonexistent-chat/messages", "0x7d273271690538cf855e5b3002a0dd8c154bb060")
		assert.Equal(t, 200, status)

		jsonAssert(t, body, map[string]any{
			"health.is_healthy": true,
		})

		// Check that the response contains an empty data array
		assert.Contains(t, string(body), `"data":[]`)
	})

	t.Run("pagination with before parameter", func(t *testing.T) {
		// Get messages before a specific time
		beforeTime := now.Add(-time.Minute * 7) // Should only get msg1 and msg2
		url := fmt.Sprintf("/comms/chats/test-chat-1/messages?before=%s", beforeTime.Format(time.RFC3339))

		status, body := testGetWithWallet(t, app, url, "0x7d273271690538cf855e5b3002a0dd8c154bb060")
		assert.Equal(t, 200, status)

		jsonAssert(t, body, map[string]any{
			"data.0.message_id": "msg2",
			"data.1.message_id": "msg1",
			"data.2":            nil, // msg3 should be filtered out
		})
	})

	t.Run("pagination with after parameter", func(t *testing.T) {
		// Get messages after a specific time
		afterTime := now.Add(-time.Minute * 7) // Should only get msg3
		url := fmt.Sprintf("/comms/chats/test-chat-1/messages?after=%s", afterTime.Format(time.RFC3339))

		status, body := testGetWithWallet(t, app, url, "0x7d273271690538cf855e5b3002a0dd8c154bb060")
		assert.Equal(t, 200, status)

		jsonAssert(t, body, map[string]any{
			"data.0.message_id": "msg3",
			"data.1":            nil, // Only msg3 should be returned
		})
	})
}

func TestGetChatMessagesWithClearedHistory(t *testing.T) {
	app := emptyTestApp(t)

	now := time.Now()
	fixtures := database.FixtureMap{
		"users": {
			{
				"user_id":    1,
				"handle":     "user1",
				"wallet":     "0x7d273271690538cf855e5b3002a0dd8c154bb060",
				"created_at": now.Add(-time.Hour),
				"updated_at": now.Add(-time.Hour),
				"is_current": true,
			},
		},
		"chat": {
			{
				"chat_id":         "test-chat-cleared",
				"created_at":      now.Add(-time.Hour),
				"last_message_at": now.Add(-time.Minute * 5),
			},
		},
		"chat_member": {
			{
				"chat_id":            "test-chat-cleared",
				"user_id":            1,
				"invited_by_user_id": 1,
				"invite_code":        "",
				"created_at":         now.Add(-time.Hour),
				"cleared_history_at": now.Add(-time.Minute * 8), // Cleared history 8 minutes ago
			},
		},
		"chat_message": {
			{
				"message_id": "old_msg",
				"chat_id":    "test-chat-cleared",
				"user_id":    1,
				"created_at": now.Add(-time.Minute * 10), // Before cleared_history_at
				"ciphertext": "Old message",
			},
			{
				"message_id": "new_msg",
				"chat_id":    "test-chat-cleared",
				"user_id":    1,
				"created_at": now.Add(-time.Minute * 5), // After cleared_history_at
				"ciphertext": "New message",
			},
		},
	}

	database.Seed(app.pool, fixtures)

	t.Run("only get messages after cleared history", func(t *testing.T) {
		status, body := testGetWithWallet(t, app, "/comms/chats/test-chat-cleared/messages", "0x7d273271690538cf855e5b3002a0dd8c154bb060")
		assert.Equal(t, 200, status)

		jsonAssert(t, body, map[string]any{
			"data.0.message_id": "new_msg",
			"data.0.message":    "New message",
			"data.1":            nil, // old_msg should be filtered out due to cleared history
			"health.is_healthy": true,
		})
	})
}
