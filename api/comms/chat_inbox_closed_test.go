package comms

import (
	"context"
	"fmt"
	"testing"
	"time"

	"api.audius.co/database"
	"api.audius.co/trashid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// An artist who blasts their followers and later closes their inbox must stop
// receiving new DMs from blast recipients. Threads that hold real messages keep
// working, and a fresh blast sent after closing re-opens replies to it.
func TestChatBlastThenCloseInbox(t *testing.T) {
	t0 := time.Now().Add(time.Second * -100).UTC()
	t1 := time.Now().Add(time.Second * -90).UTC()
	t2 := time.Now().Add(time.Second * -80).UTC()
	t3 := time.Now().Add(time.Second * -70).UTC()
	t4 := time.Now().Add(time.Second * -60).UTC()
	t5 := time.Now().Add(time.Second * -50).UTC()

	pool := database.CreateTestDatabase(t, "test_comms")
	defer pool.Close()
	ctx := context.Background()

	// user 1 is the artist. 201, 202, 203 follow them before the blast.
	database.Seed(pool, database.FixtureMap{
		"users": {
			{"user_id": 1, "wallet": "wallet1", "handle": "user1"},
			{"user_id": 201, "wallet": "wallet201", "handle": "user201"},
			{"user_id": 202, "wallet": "wallet202", "handle": "user202"},
			{"user_id": 203, "wallet": "wallet203", "handle": "user203"},
		},
		"follows": {
			{"follower_user_id": 201, "followee_user_id": 1, "created_at": t0},
			{"follower_user_id": 202, "followee_user_id": 1, "created_at": t0},
			{"follower_user_id": 203, "followee_user_id": 1, "created_at": t0},
		},
	})
	validator := CreateTestValidator(t, pool, DefaultRateLimitConfig, DefaultTestValidatorConfig)

	chatAllowed := func(from, to int32) bool {
		var ok bool
		require.NoError(t, pool.QueryRow(ctx, `select chat_allowed($1, $2)`, from, to).Scan(&ok))
		return ok
	}
	assertMessageAllowed := func(sender int32, chatId string, shouldWork bool) {
		rpc := RawRPC{
			Params: []byte(fmt.Sprintf(`{"chat_id": "%s", "message_id": "m-%s-%d", "message": "hi"}`, chatId, chatId, sender)),
		}
		err := validator.validateChatMessage(ctx, sender, rpc)
		if shouldWork {
			assert.NoError(t, err)
		} else {
			assert.ErrorContains(t, err, "Not permitted to send messages to this user")
		}
	}
	upgrade := func(follower int32, ts time.Time) string {
		chatId := trashid.ChatID(int(follower), 1)
		err := chatCreate(pool, ctx, follower, ts, ChatCreateRPCParams{
			ChatID: chatId,
			Invites: []PurpleInvite{
				{UserID: trashid.MustEncodeHashID(int(follower)), InviteCode: "x"},
				{UserID: trashid.MustEncodeHashID(1), InviteCode: "x"},
			},
		})
		require.NoError(t, err)
		return chatId
	}

	// artist blasts followers with an open inbox
	_, err := chatBlast(pool, ctx, 1, t1, ChatBlastRPCParams{
		BlastID:  "b_open",
		Audience: FollowerAudience,
		Message:  "hello followers",
	})
	require.NoError(t, err)

	// 202 opens the blast into a thread but never replies
	chatId_202 := upgrade(202, t2)

	// 203 opens the blast into a thread and replies, so a real conversation exists
	chatId_203 := upgrade(203, t2)
	require.NoError(t, chatSendMessage(pool, ctx, 203, chatId_203, "reply_203", t2, "203 replying"))

	// while the inbox is open, everyone in the audience can reach the artist
	assertChatCreateAllowed(t, ctx, validator, 201, 1, true)
	assertMessageAllowed(202, chatId_202, true)
	assertMessageAllowed(203, chatId_203, true)

	// artist closes their inbox
	require.NoError(t, chatSetPermissions(pool, ctx, 1, ChatPermissionAll, []ChatPermission{ChatPermissionNone}, boolPtr(true), t3))

	// 201 can no longer start a thread off the old blast
	assertChatCreateAllowed(t, ctx, validator, 201, 1, false)
	assert.False(t, chatAllowed(201, 1))

	// 202's thread holds nothing but the blast seed, so it grants no reply rights
	assertMessageAllowed(202, chatId_202, false)
	assert.False(t, chatAllowed(202, 1))

	// 203's thread is a real conversation and keeps working
	assertMessageAllowed(203, chatId_203, true)
	assert.True(t, chatAllowed(203, 1))

	// a follower's own view of the seed-only thread asks the client to recheck permissions
	{
		chat, err := getUserChat(pool, ctx, chatMembershipParams{UserID: 202, ChatID: chatId_202})
		require.NoError(t, err)
		assert.True(t, chat.LastMessageIsPlaintext)
	}

	// artist blasts again while closed: recipients of the new blast may reply to it
	_, err = chatBlast(pool, ctx, 1, t4, ChatBlastRPCParams{
		BlastID:  "b_closed",
		Audience: FollowerAudience,
		Message:  "hello again",
	})
	require.NoError(t, err)

	assertChatCreateAllowed(t, ctx, validator, 201, 1, true)
	assert.True(t, chatAllowed(202, 1), "new blast fanned into 202's thread re-opens replies")

	// closing the inbox once more after that blast shuts the door again
	require.NoError(t, chatSetPermissions(pool, ctx, 1, ChatPermissionAll, []ChatPermission{ChatPermissionNone}, boolPtr(true), t5))
	assertChatCreateAllowed(t, ctx, validator, 201, 1, false)
	assert.False(t, chatAllowed(202, 1))
	assert.True(t, chatAllowed(203, 1))
}

func boolPtr(b bool) *bool { return &b }
