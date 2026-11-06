package comms

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"api.audius.co/database"
	"api.audius.co/trashid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testGroupChatID = "grp_abcdefghijklmnop1234"

func groupCreateRPC(t *testing.T, chatId string, userIds ...int32) RawRPC {
	invites := make([]PurpleInvite, 0, len(userIds))
	for _, id := range userIds {
		invites = append(invites, PurpleInvite{UserID: trashid.MustEncodeHashID(int(id)), InviteCode: "code"})
	}
	params, err := json.Marshal(ChatCreateRPCParams{ChatID: chatId, Invites: invites})
	require.NoError(t, err)
	return RawRPC{Method: string(RPCMethodChatCreate), Params: params}
}

func chatRPC(chatId string) RawRPC {
	return RawRPC{Params: []byte(fmt.Sprintf(`{"chat_id": "%s", "message_id": "m", "message": "hi"}`, chatId))}
}

func TestIsGroupChatID(t *testing.T) {
	assert.True(t, IsGroupChatID(testGroupChatID))
	assert.False(t, IsGroupChatID("grp_short"))
	assert.False(t, IsGroupChatID("grp_abcdefghijklmnop:1234"))
	assert.False(t, IsGroupChatID(trashid.ChatID(1, 2)))
	assert.False(t, IsGroupChatID("grp_"+strings.Repeat("a", 65)))
}

func TestGroupChatCreateValidation(t *testing.T) {
	pool := database.CreateTestDatabase(t, "test_comms")
	defer pool.Close()
	ctx := context.Background()

	database.Seed(pool, database.FixtureMap{
		"users": {
			{"user_id": 1, "wallet": "wallet1", "handle": "user1"},
			{"user_id": 2, "wallet": "wallet2", "handle": "user2"},
			{"user_id": 3, "wallet": "wallet3", "handle": "user3"},
			{"user_id": 4, "wallet": "wallet4", "handle": "user4"},
		},
	})
	validator := CreateTestValidator(t, pool, DefaultRateLimitConfig, DefaultTestValidatorConfig)

	// valid group
	assert.NoError(t, validator.validateChatCreate(ctx, 1, groupCreateRPC(t, testGroupChatID, 1, 2, 3)))

	// group needs a group id, and 1:1 can't use one
	assert.ErrorContains(t, validator.validateChatCreate(ctx, 1, groupCreateRPC(t, "a:b:c", 1, 2, 3)), "Invalid chat id")
	assert.ErrorContains(t, validator.validateChatCreate(ctx, 1, groupCreateRPC(t, testGroupChatID, 1, 2)), "Invalid chat id")

	// creator must be in the chat, members unique
	assert.ErrorContains(t, validator.validateChatCreate(ctx, 1, groupCreateRPC(t, testGroupChatID, 2, 3, 4)), "creator must be a member")
	assert.ErrorContains(t, validator.validateChatCreate(ctx, 1, groupCreateRPC(t, testGroupChatID, 1, 2, 2)), "unique")
	assert.ErrorContains(t, validator.validateChatCreate(ctx, 1, groupCreateRPC(t, testGroupChatID, 1)), "at least 2")

	// member cap
	tooMany := []int32{}
	for i := int32(1); i <= MaxGroupChatMembers+1; i++ {
		tooMany = append(tooMany, i)
	}
	assert.ErrorContains(t, validator.validateChatCreate(ctx, 1, groupCreateRPC(t, testGroupChatID, tooMany...)), "at most")

	// every invitee's inbox settings apply to the creator
	require.NoError(t, chatSetPermissions(pool, ctx, 3, ChatPermissionNone, nil, nil, time.Now()))
	assert.ErrorContains(t, validator.validateChatCreate(ctx, 1, groupCreateRPC(t, testGroupChatID, 1, 2, 3)), "Not permitted")
	assert.NoError(t, validator.validateChatCreate(ctx, 1, groupCreateRPC(t, testGroupChatID, 1, 2, 4)))

	// blocks between creator and an invitee apply too
	require.NoError(t, chatBlock(pool, ctx, 4, 1, time.Now()))
	assert.ErrorContains(t, validator.validateChatCreate(ctx, 1, groupCreateRPC(t, testGroupChatID, 1, 2, 4)), "Not permitted")
}

func TestGroupChatMessagingAndLeave(t *testing.T) {
	pool := database.CreateTestDatabase(t, "test_comms")
	defer pool.Close()
	ctx := context.Background()

	database.Seed(pool, database.FixtureMap{
		"users": {
			{"user_id": 1, "wallet": "wallet1", "handle": "user1"},
			{"user_id": 2, "wallet": "wallet2", "handle": "user2"},
			{"user_id": 3, "wallet": "wallet3", "handle": "user3"},
			{"user_id": 4, "wallet": "wallet4", "handle": "user4"},
		},
		// user 3 only accepts DMs from accounts they follow; they follow user 1
		"follows": {
			{"follower_user_id": 3, "followee_user_id": 1},
		},
	})
	require.NoError(t, chatSetPermissions(pool, ctx, 3, ChatPermissionFollowees, nil, nil, time.Now().Add(-time.Hour)))
	validator := CreateTestValidator(t, pool, DefaultRateLimitConfig, DefaultTestValidatorConfig)

	ts := time.Now().Add(-time.Minute)
	params := ChatCreateRPCParams{ChatID: testGroupChatID}
	for _, id := range []int{1, 2, 3} {
		params.Invites = append(params.Invites, PurpleInvite{UserID: trashid.MustEncodeHashID(id), InviteCode: "code"})
	}
	require.NoError(t, validator.validateChatCreate(ctx, 1, groupCreateRPC(t, testGroupChatID, 1, 2, 3)))
	require.NoError(t, chatCreate(pool, ctx, 1, ts, params))

	var isGroup bool
	require.NoError(t, pool.QueryRow(ctx, `select is_group from chat where chat_id = $1`, testGroupChatID).Scan(&isGroup))
	assert.True(t, isGroup)

	// user 2 can post even though user 3's settings would reject a 1:1 from them
	assert.NoError(t, validator.validateChatMessage(ctx, 2, chatRPC(testGroupChatID)))
	require.NoError(t, chatSendMessage(pool, ctx, 2, testGroupChatID, "msg1", ts.Add(time.Second), "hello group"))

	// non-members can't post
	assert.ErrorContains(t, validator.validateChatMessage(ctx, 4, chatRPC(testGroupChatID)), "not a member")

	// sharing a group does not open a 1:1 between 2 and 3
	var allowed bool
	require.NoError(t, pool.QueryRow(ctx, `select chat_allowed(2, 3)`).Scan(&allowed))
	assert.False(t, allowed)
	require.NoError(t, pool.QueryRow(ctx, `select chat_allowed(1, 3)`).Scan(&allowed))
	assert.True(t, allowed)

	// unread counts go to everyone but the sender
	unread := func(userId int32) int {
		var n int
		require.NoError(t, pool.QueryRow(ctx, `select unread_count from chat_member where chat_id = $1 and user_id = $2`, testGroupChatID, userId).Scan(&n))
		return n
	}
	assert.Equal(t, 0, unread(2))
	assert.Equal(t, 1, unread(1))
	assert.Equal(t, 1, unread(3))

	// messages from a blocked member don't count as unread for the blocker
	require.NoError(t, chatBlock(pool, ctx, 3, 2, ts.Add(2*time.Second)))
	require.NoError(t, chatSendMessage(pool, ctx, 2, testGroupChatID, "msg2", ts.Add(3*time.Second), "again"))
	assert.Equal(t, 2, unread(1))
	assert.Equal(t, 0, unread(3))
	// and the blocked member can still post to the rest of the group
	assert.NoError(t, validator.validateChatMessage(ctx, 2, chatRPC(testGroupChatID)))

	// leave: only groups, only members
	leaveRPC := RawRPC{Params: []byte(fmt.Sprintf(`{"chat_id": "%s"}`, testGroupChatID))}
	assert.ErrorContains(t, validator.validateChatLeave(ctx, 4, leaveRPC), "not a member")
	assert.NoError(t, validator.validateChatLeave(ctx, 3, leaveRPC))
	require.NoError(t, chatSetCategory(pool, ctx, 3, testGroupChatID, strPtr("general"), ts))
	require.NoError(t, chatLeave(pool, ctx, 3, testGroupChatID))

	var members, prefs int
	require.NoError(t, pool.QueryRow(ctx, `select count(*) from chat_member where chat_id = $1`, testGroupChatID).Scan(&members))
	require.NoError(t, pool.QueryRow(ctx, `select count(*) from user_conversation_preferences where chat_id = $1`, testGroupChatID).Scan(&prefs))
	assert.Equal(t, 2, members)
	assert.Equal(t, 0, prefs)
	assert.ErrorContains(t, validator.validateChatMessage(ctx, 3, chatRPC(testGroupChatID)), "not a member")

	// still a group with two members left, so posting keeps working
	assert.NoError(t, validator.validateChatMessage(ctx, 1, chatRPC(testGroupChatID)))

	// 1:1 chats can't be left
	oneToOne := trashid.ChatID(1, 2)
	SetupChatWithMembers(t, pool, ctx, oneToOne, 1, 2, "a", "b")
	assert.ErrorContains(t, validator.validateChatLeave(ctx, 1, RawRPC{Params: []byte(fmt.Sprintf(`{"chat_id": "%s"}`, oneToOne))}), "Only group chats")
}

func TestBlastSkipsGroupChats(t *testing.T) {
	pool := database.CreateTestDatabase(t, "test_comms")
	defer pool.Close()
	ctx := context.Background()

	t0 := time.Now().Add(-time.Hour).UTC()
	database.Seed(pool, database.FixtureMap{
		"users": {
			{"user_id": 1, "wallet": "wallet1", "handle": "user1"},
			{"user_id": 2, "wallet": "wallet2", "handle": "user2"},
			{"user_id": 3, "wallet": "wallet3", "handle": "user3"},
		},
		"follows": {
			{"follower_user_id": 2, "followee_user_id": 1, "created_at": t0},
		},
	})

	params := ChatCreateRPCParams{ChatID: testGroupChatID}
	for _, id := range []int{1, 2, 3} {
		params.Invites = append(params.Invites, PurpleInvite{UserID: trashid.MustEncodeHashID(id), InviteCode: "code"})
	}
	require.NoError(t, chatCreate(pool, ctx, 1, t0, params))
	require.NoError(t, chatSendMessage(pool, ctx, 1, testGroupChatID, "msg1", t0.Add(time.Second), "hi"))

	// user 1 blasts followers; user 2 shares only a group with user 1
	_, err := chatBlast(pool, ctx, 1, time.Now().UTC(), ChatBlastRPCParams{
		BlastID:  "blast1",
		Audience: FollowerAudience,
		Message:  "new track",
	})
	require.NoError(t, err)

	var n int
	require.NoError(t, pool.QueryRow(ctx, `select count(*) from chat_message where chat_id = $1 and blast_id is not null`, testGroupChatID).Scan(&n))
	assert.Equal(t, 0, n)
}

func strPtr(s string) *string { return &s }
