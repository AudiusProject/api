package comms

import (
	"context"
	"fmt"
	"testing"
	"time"

	"bridgerton.audius.co/database"
	"bridgerton.audius.co/trashid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
)

/*
   Note: There is some overlap between these tests and those in comms_blasts_test.go
   These tests are meant to exercise the write path.
*/

func mustGetMessagesAndReactions(t *testing.T, pool *pgxpool.Pool, ctx context.Context, userID int32, chatID string) []chatMessageAndReactionsRow {
	messages, err := getChatMessagesAndReactions(pool, ctx, chatMessagesAndReactionsParams{
		UserID: userID,
		ChatID: chatID,
		Limit:  10,
		Before: time.Now().Add(time.Hour * 2).UTC(),
		After:  time.Now().Add(time.Hour * -2).UTC(),
	})
	assert.NoError(t, err)
	return messages
}

func TestChatBlastFollowers(t *testing.T) {
	t0 := time.Now().Add(time.Second * -100).UTC()
	t1 := time.Now().Add(time.Second * -90).UTC()
	t2 := time.Now().Add(time.Second * -80).UTC()
	t3 := time.Now().Add(time.Second * -70).UTC()
	t4 := time.Now().Add(time.Second * -60).UTC()
	t5 := time.Now().Add(time.Second * -50).UTC()
	t6 := time.Now().Add(time.Second * -40).UTC()

	pool := database.CreateTestDatabase(t, "test_comms")
	defer pool.Close()
	database.Seed(pool, database.FixtureMap{
		"users": {
			{"user_id": 68, "wallet": "wallet68", "handle": "user68"},
			{"user_id": 1, "wallet": "wallet1", "handle": "user1"},
			{"user_id": 100, "wallet": "wallet100", "handle": "user100"},
			{"user_id": 101, "wallet": "wallet101", "handle": "user101"},
			{"user_id": 102, "wallet": "wallet102", "handle": "user102"},
			{"user_id": 103, "wallet": "wallet103", "handle": "user103"},
			{"user_id": 104, "wallet": "wallet104", "handle": "user104"},
		},
		"follows": {{
			"follower_user_id": 68,
			"followee_user_id": 1,
			"created_at":       t0,
		}, {
			"follower_user_id": 1,
			"followee_user_id": 68,
			"created_at":       t0,
		},
			{
				"follower_user_id": 100,
				"followee_user_id": 1,
				"created_at":       t0,
			},
			{
				"follower_user_id": 101,
				"followee_user_id": 1,
				"created_at":       t0,
			},
			{
				"follower_user_id": 102,
				"followee_user_id": 1,
				"created_at":       t0,
			},
			{
				"follower_user_id": 103,
				"followee_user_id": 1,
				"created_at":       t0,
			},
			{
				"follower_user_id": 104,
				"followee_user_id": 1,
				"created_at":       t0,
			},
		},
	})
	validator := CreateTestValidator(t, pool, DefaultRateLimitConfig, DefaultTestValidatorConfig)

	ctx := context.Background()

	// TODO: Scoped
	var count = 0
	var messages []chatMessageAndReactionsRow

	// Blaster (user 1) closes inbox
	// But recipients should still be able to upgrade.
	err := chatSetPermissions(pool, ctx, 1, ChatPermissionNone, nil, nil, t0)
	assert.NoError(t, err)

	// Other user (104) closes inbox
	err = chatSetPermissions(pool, ctx, 104, ChatPermissionNone, nil, nil, t0)
	assert.NoError(t, err)

	// ----------------- some threads already exist -------------
	// user 100 starts a thread with 1 before first blast
	chatId_100_1 := trashid.ChatID(100, 1)
	chatId_1_103 := trashid.ChatID(1, 103)
	{
		err := chatCreate(pool, ctx, 100, t1, ChatCreateRPCParams{
			ChatID: chatId_100_1,
			Invites: []PurpleInvite{
				{UserID: trashid.MustEncodeHashID(100), InviteCode: "x"},
				{UserID: trashid.MustEncodeHashID(1), InviteCode: "x"},
			},
		})
		assert.NoError(t, err)

		// send a message in chat
		err = chatSendMessage(pool, ctx, 100, chatId_100_1, "pre1", t1, "100 here sending 1 a message")
		assert.NoError(t, err)

		messages = mustGetMessagesAndReactions(t, pool, ctx, 100, chatId_100_1)
		assert.Len(t, messages, 1)
		assert.False(t, messages[0].IsPlaintext)

		messages = mustGetMessagesAndReactions(t, pool, ctx, 1, chatId_100_1)
		assert.Len(t, messages, 1)

		ch, err := getUserChat(pool, ctx, chatMembershipParams{
			UserID: 1,
			ChatID: chatId_100_1,
		})
		assert.NoError(t, err)
		assert.False(t, ch.LastMessageIsPlaintext)

		// user 1 now has 1 (real) chats
		chats, err := getUserChats(pool, ctx, userChatsParams{
			UserID: 1,
			Limit:  10,
			Before: time.Now().Add(time.Hour * 2).UTC(),
			After:  time.Now().Add(time.Hour * -2).UTC(),
		})
		assert.NoError(t, err)
		assert.Len(t, chats, 1)
	}

	// user 1 starts empty thread with 103 before first blast
	{
		err := chatCreate(pool, ctx, 1, t1, ChatCreateRPCParams{
			ChatID: chatId_1_103,
			Invites: []PurpleInvite{
				{UserID: trashid.MustEncodeHashID(1), InviteCode: "x"},
				{UserID: trashid.MustEncodeHashID(103), InviteCode: "x"},
			},
		})
		assert.NoError(t, err)

		// user 1 still has 1 (real) chats
		// because this is empty
		chats, err := getUserChats(pool, ctx, userChatsParams{
			UserID: 1,
			Limit:  10,
			Before: time.Now().Add(time.Hour * 2).UTC(),
			After:  time.Now().Add(time.Hour * -2).UTC(),
		})
		assert.NoError(t, err)
		assert.Len(t, chats, 1)
	}

	// ----------------- a first blast ------------------------
	chatId_101_1 := trashid.ChatID(101, 1)

	outgoingMessages, err := chatBlast(pool, ctx, 1, t2, ChatBlastRPCParams{
		BlastID:  "b1",
		Audience: FollowerAudience,
		Message:  "what up fam",
	})
	assert.NoError(t, err)

	// Test that outgoing messages contain the audience field
	for _, outgoingMsg := range outgoingMessages {
		assert.NotNil(t, outgoingMsg.ChatMessageRPC.Params.Audience, "Audience should be set in outgoing message")
		assert.Equal(t, FollowerAudience, *outgoingMsg.ChatMessageRPC.Params.Audience, "Audience should match the blast audience")
	}

	pool.QueryRow(ctx, `select count(*) from chat_blast`).Scan(&count)
	assert.Equal(t, 1, count)

	pool.QueryRow(ctx, `select count(*) from chat where chat_id = $1`, chatId_101_1).Scan(&count)
	assert.Equal(t, 0, count)

	pool.QueryRow(ctx, `select count(*) from chat_member where chat_id = $1`, chatId_101_1).Scan(&count)
	assert.Equal(t, 0, count)

	pool.QueryRow(ctx, `select count(*) from chat_message where chat_id = $1`, chatId_101_1).Scan(&count)
	assert.Equal(t, 0, count)

	// user 1 gets chat list...
	{
		// user 1 now has a (pre-existing) chat and a blast
		chats, err := getUserChats(pool, ctx, userChatsParams{
			UserID: 1,
			Limit:  10,
			Before: time.Now().Add(time.Hour * 2).UTC(),
			After:  time.Now().Add(time.Hour * -2).UTC(),
		})
		assert.NoError(t, err)
		assert.Len(t, chats, 2)

		blastCount := 0
		for _, c := range chats {
			if c.IsBlast {
				blastCount++
			}
		}
		assert.Equal(t, "7eP5n:eYZmn", chats[1].ChatID)
		assert.Equal(t, 1, blastCount)
	}

	// user 100 (pre-existing) has a new message, but no new blasts
	{
		blasts, err := getNewBlasts(pool, ctx, getNewBlastsParams{
			UserID: 100,
		})
		assert.NoError(t, err)
		assert.Len(t, blasts, 0)

		messages = mustGetMessagesAndReactions(t, pool, ctx, 100, chatId_100_1)
		assert.Len(t, messages, 2)

		messages = mustGetMessagesAndReactions(t, pool, ctx, 1, chatId_100_1)
		assert.Len(t, messages, 2)
	}

	// user 103 (pre-existing) has a new message, but no new blasts
	{
		blasts, err := getNewBlasts(pool, ctx, getNewBlastsParams{
			UserID: 103,
		})
		assert.NoError(t, err)
		assert.Len(t, blasts, 0)

		messages = mustGetMessagesAndReactions(t, pool, ctx, 103, chatId_1_103)
		assert.Len(t, messages, 1)

		messages = mustGetMessagesAndReactions(t, pool, ctx, 1, chatId_1_103)
		assert.Len(t, messages, 1)
	}

	// user 101 has a blast
	{
		blasts, err := getNewBlasts(pool, ctx, getNewBlastsParams{
			UserID: 101,
		})
		assert.NoError(t, err)
		assert.Len(t, blasts, 1)
	}

	// user 104 has zero blasts (inbox closed)
	{
		blasts, err := getNewBlasts(pool, ctx, getNewBlastsParams{
			UserID: 104,
		})
		assert.NoError(t, err)
		assert.Len(t, blasts, 0)
	}

	// user 999 does not
	{
		assertChatCreateAllowed(t, ctx, validator, 999, 1, false)

		blasts, err := getNewBlasts(pool, ctx, getNewBlastsParams{
			UserID: 999,
		})
		assert.NoError(t, err)
		assert.Len(t, blasts, 0)
	}

	// user 101 upgrades it to a real DM
	{

		assertChatCreateAllowed(t, ctx, validator, 101, 1, true)

		err = chatCreate(pool, ctx, 101, t3, ChatCreateRPCParams{
			ChatID: chatId_101_1,
			Invites: []PurpleInvite{
				{UserID: trashid.MustEncodeHashID(101), InviteCode: "earlier"},
				{UserID: trashid.MustEncodeHashID(1), InviteCode: "earlier"},
			},
		})
		assert.NoError(t, err)

		pool.QueryRow(ctx, `select count(*) from chat where chat_id = $1`, chatId_101_1).Scan(&count)
		assert.Equal(t, 1, count)

		pool.QueryRow(ctx, `select count(*) from chat_member where chat_id = $1`, chatId_101_1).Scan(&count)
		assert.Equal(t, 2, count)

		pool.QueryRow(ctx, `select count(*) from chat_member where is_hidden = false and chat_id = $1 and user_id = 101`, chatId_101_1).Scan(&count)
		assert.Equal(t, 1, count)

		pool.QueryRow(ctx, `select count(*) from chat_member where is_hidden = true and chat_id = $1 and user_id = 1`, chatId_101_1).Scan(&count)
		assert.Equal(t, 1, count)

		pool.QueryRow(ctx, `select count(*) from chat_message where chat_id = $1`, chatId_101_1).Scan(&count)
		assert.Equal(t, 1, count)

		messages = mustGetMessagesAndReactions(t, pool, ctx, 101, chatId_101_1)
		assert.Len(t, messages, 1)
	}

	// after upgrade... user 101 has no pending blasts
	{
		blasts, err := getNewBlasts(pool, ctx, getNewBlastsParams{
			UserID: 101,
		})
		assert.NoError(t, err)
		assert.Len(t, blasts, 0)
	}

	// after upgrade... user 101 has a chat
	{
		chats, err := getUserChats(pool, ctx, userChatsParams{
			UserID: 101,
			Limit:  10,
			Before: time.Now().Add(time.Hour * 12),
			After:  time.Now().Add(time.Hour * -12),
		})
		assert.NoError(t, err)
		assert.Len(t, chats, 1)
	}

	// after upgrade... user 1 doesn't actually see the chat because it is hidden
	{
		chats, err := getUserChats(pool, ctx, userChatsParams{
			UserID: 1,
			Limit:  50,
			Before: time.Now().Add(time.Hour * 12),
			After:  time.Now().Add(time.Hour * -12),
		})
		assert.NoError(t, err)
		for _, chat := range chats {
			if chat.ChatID == chatId_101_1 {
				assert.Fail(t, "chat id should be hidden from user 1", chatId_101_1)
			}
		}
	}

	// artist view: user 1 can get this blast
	{
		chat, err := getUserChat(pool, ctx, chatMembershipParams{
			UserID: 1,
			ChatID: string(FollowerAudience),
		})
		assert.NoError(t, err)
		assert.Equal(t, string(FollowerAudience), chat.ChatID)
	}

	// ----------------- a second message ------------------------

	// Other user (104) re-opens inbox
	err = chatSetPermissions(pool, ctx, 104, ChatPermissionAll, nil, nil, t3)
	assert.NoError(t, err)

	outgoingMessages2, err := chatBlast(pool, ctx, 1, t4, ChatBlastRPCParams{
		BlastID:  "b2",
		Audience: FollowerAudience,
		Message:  "happy wed",
	})
	assert.NoError(t, err)

	// Test that second blast also includes audience field
	for _, outgoingMsg := range outgoingMessages2 {
		assert.NotNil(t, outgoingMsg.ChatMessageRPC.Params.Audience, "Audience should be set in second blast outgoing message")
		assert.Equal(t, FollowerAudience, *outgoingMsg.ChatMessageRPC.Params.Audience, "Audience should match the blast audience")
	}

	pool.QueryRow(ctx, `select count(*) from chat_blast`).Scan(&count)
	assert.Equal(t, 2, count)

	// user 101 above should have second blast added to the chat history...
	{
		chatId := trashid.ChatID(101, 1)

		pool.QueryRow(ctx, `select count(*) from chat_message where chat_id = $1`, chatId).Scan(&count)
		assert.Equal(t, 2, count)

		messages = mustGetMessagesAndReactions(t, pool, ctx, 1, chatId)
		assert.Len(t, messages, 2)

		assert.Equal(t, "happy wed", messages[0].Ciphertext)
		assert.True(t, messages[0].IsPlaintext)
		assert.Equal(t, "what up fam", messages[1].Ciphertext)
		assert.True(t, messages[1].IsPlaintext)
		assert.Greater(t, messages[0].CreatedAt, messages[1].CreatedAt)

		ch, err := getUserChat(pool, ctx, chatMembershipParams{
			UserID: 1,
			ChatID: chatId,
		})
		assert.NoError(t, err)
		assert.True(t, ch.LastMessageIsPlaintext)
		assert.Equal(t, "happy wed", ch.LastMessage.String)

		// user 101 reacts
		{
			heart := "heart"
			chatReactMessage(pool, ctx, 101, chatId, messages[0].MessageID, &heart, t5)

			// reaction shows up
			messages = mustGetMessagesAndReactions(t, pool, ctx, 1, chatId)
			assert.Equal(t, "heart", messages[0].Reactions[0].Reaction)
		}

		if false {
			var debugRows []string
			rows, err := pool.Query(ctx, `select row_to_json(c) from chat c;`)
			assert.NoError(t, err)
			defer rows.Close()
			for rows.Next() {
				var d string
				err := rows.Scan(&debugRows)
				assert.NoError(t, err)
				fmt.Println("CHAT:", d)
			}
		}

	}

	// user 101 replies... now user 1 should see the thread
	{
		err = chatSendMessage(pool, ctx, 101, chatId_101_1, "respond_to_blast", t6, "101 responding to a blast from 1")
		assert.NoError(t, err)

		chats, err := getUserChats(pool, ctx, userChatsParams{
			UserID: 1,
			Limit:  50,
			Before: time.Now().Add(time.Hour * 12),
			After:  time.Now().Add(time.Hour * -12),
		})
		assert.NoError(t, err)
		found := false
		for _, chat := range chats {
			if chat.ChatID == chatId_101_1 {
				found = true
				break
			}
		}
		if !found {
			assert.Fail(t, "chat id should now be visible to user 1", chatId_101_1)
		}
	}

	// user 104 should have just 1 blast
	// since 104 opened inbox after first blast
	{
		blasts, err := getNewBlasts(pool, ctx, getNewBlastsParams{
			UserID: 104,
		})
		assert.NoError(t, err)
		assert.Len(t, blasts, 1)

		// 104 does upgrade
		chatId_104_1 := trashid.ChatID(104, 1)

		err = chatCreate(pool, ctx, 104, t6, ChatCreateRPCParams{
			ChatID: chatId_104_1,
			Invites: []PurpleInvite{
				{UserID: trashid.MustEncodeHashID(104), InviteCode: "earlier"},
				{UserID: trashid.MustEncodeHashID(1), InviteCode: "earlier"},
			},
		})
		assert.NoError(t, err)

		// 104 convo seeded with 1 message

		messages := mustGetMessagesAndReactions(t, pool, ctx, 104, chatId_104_1)
		assert.Len(t, messages, 1)
		messages = mustGetMessagesAndReactions(t, pool, ctx, 1, chatId_104_1)
		assert.Len(t, messages, 1)
	}

	// ------ sender can get blasts in a given thread ----------
	{
		chat, err := getUserChat(pool, ctx, chatMembershipParams{
			UserID: 1,
			ChatID: string(FollowerAudience),
		})
		assert.NoError(t, err)
		assert.Equal(t, string(FollowerAudience), chat.ChatID)

		messages, err := getChatMessagesAndReactions(pool, ctx, chatMessagesAndReactionsParams{
			UserID:  1,
			ChatID:  "follower_audience",
			IsBlast: true,
			Before:  time.Now().Add(time.Hour * 2).UTC(),
			After:   time.Now().Add(time.Hour * -2).UTC(),
			Limit:   10,
		})
		assert.NoError(t, err)
		assert.Len(t, messages, 2)
	}

	// ------- bi-directional blasting works with upgrade --------

	// 1 re-opens inbox
	err = chatSetPermissions(pool, ctx, 1, ChatPermissionAll, nil, nil, t1)
	assert.NoError(t, err)

	// 68 sends a blast
	chatId_68_1 := trashid.ChatID(68, 1)

	_, err = chatBlast(pool, ctx, 68, t4, ChatBlastRPCParams{
		BlastID:  "blast_from_68",
		Audience: FollowerAudience,
		Message:  "I am 68",
	})
	assert.NoError(t, err)

	// one side does upgrade
	err = chatCreate(pool, ctx, 1, t5, ChatCreateRPCParams{
		ChatID: chatId_68_1,
		Invites: []PurpleInvite{
			{UserID: trashid.MustEncodeHashID(68), InviteCode: "earlier"},
			{UserID: trashid.MustEncodeHashID(1), InviteCode: "earlier"},
		},
	})
	assert.NoError(t, err)

	// both parties should have 3 messages message
	{
		messages := mustGetMessagesAndReactions(t, pool, ctx, 68, chatId_68_1)
		assert.Len(t, messages, 3)
	}

	// both parties should have 3 messages message
	{
		messages := mustGetMessagesAndReactions(t, pool, ctx, 1, chatId_68_1)
		assert.Len(t, messages, 3)
	}
}

func TestChatBlastTippers(t *testing.T) {
	pool := database.CreateTestDatabase(t, "test_comms")
	defer pool.Close()
	database.Seed(pool, database.FixtureMap{
		"users": {
			{"user_id": 1, "wallet": "wallet1", "handle": "user1"},
			{"user_id": 201, "wallet": "wallet201", "handle": "user201"},
		},
		"user_tips": {
			{
				"sender_user_id":   201,
				"receiver_user_id": 1,
				"amount":           1000,
				"slot":             101,
				"signature":        "tip_sig_123",
			},
		},
	})

	ctx := context.Background()

	// 1 sends blast to supporters
	tipperOutgoing, err := chatBlast(pool, ctx, 1, time.Now().UTC(), ChatBlastRPCParams{
		BlastID:  "blast_tippers_1",
		Audience: TipperAudience,
		Message:  "thanks for your support",
	})
	assert.NoError(t, err)

	// Test that tipper blast includes correct audience field
	for _, outgoingMsg := range tipperOutgoing {
		assert.NotNil(t, outgoingMsg.ChatMessageRPC.Params.Audience, "Audience should be set in tipper blast outgoing message")
		assert.Equal(t, TipperAudience, *outgoingMsg.ChatMessageRPC.Params.Audience, "Audience should match the tipper audience")
	}

	// 201 should have a pending blast
	{
		pending, err := getNewBlasts(pool, ctx, getNewBlastsParams{
			UserID: 201,
		})
		assert.NoError(t, err)
		assert.Len(t, pending, 1)
	}

	// 1 upgrades
	chatId_1_201 := trashid.ChatID(1, 201)
	err = chatCreate(pool, ctx, 101, time.Now().UTC(), ChatCreateRPCParams{
		ChatID: chatId_1_201,
		Invites: []PurpleInvite{
			{UserID: trashid.MustEncodeHashID(1), InviteCode: "earlier"},
			{UserID: trashid.MustEncodeHashID(201), InviteCode: "earlier"},
		},
	})
	assert.NoError(t, err)

	// both users have 1 message
	{
		messages := mustGetMessagesAndReactions(t, pool, ctx, 1, chatId_1_201)
		assert.Len(t, messages, 1)
	}
	{
		messages := mustGetMessagesAndReactions(t, pool, ctx, 201, chatId_1_201)
		assert.Len(t, messages, 1)
	}

	// 201 should have no pending blast
	{
		pending, err := getNewBlasts(pool, ctx, getNewBlastsParams{
			UserID: 201,
		})
		assert.NoError(t, err)
		assert.Len(t, pending, 0)
	}

	{
		chat, err := getUserChat(pool, ctx, chatMembershipParams{
			UserID: 1,
			ChatID: string(TipperAudience),
		})
		assert.NoError(t, err)
		assert.Equal(t, string(TipperAudience), chat.ChatID)
	}
}

func TestChatBlastRemixers(t *testing.T) {
	trackContentType := AudienceContentType("track")
	pool := database.CreateTestDatabase(t, "test_comms")
	defer pool.Close()
	database.Seed(pool, database.FixtureMap{
		"users": {
			{"user_id": 1, "wallet": "wallet1", "handle": "user1"},
			{"user_id": 202, "wallet": "wallet202", "handle": "user202"},
		},
		"tracks": {
			{
				"track_id": 1,
				"owner_id": 1,
			},
			{
				"track_id": 2,
				"owner_id": 202,
			},
		},
		"remixes": {
			{
				"parent_track_id": 1,
				"child_track_id":  2,
			},
		},
	})

	ctx := context.Background()

	// 1 sends blast to remixers
	remixerOutgoing, err := chatBlast(pool, ctx, 1, time.Now().UTC(), ChatBlastRPCParams{
		BlastID:             "blast_remixers_1",
		Audience:            RemixerAudience,
		AudienceContentType: &trackContentType,
		AudienceContentID:   stringPointer(trashid.MustEncodeHashID(1)),
		Message:             "thanks for your remix",
	})
	assert.NoError(t, err)

	// Test that remixer blast includes correct audience field
	for _, outgoingMsg := range remixerOutgoing {
		assert.NotNil(t, outgoingMsg.ChatMessageRPC.Params.Audience, "Audience should be set in remixer blast outgoing message")
		assert.Equal(t, RemixerAudience, *outgoingMsg.ChatMessageRPC.Params.Audience, "Audience should match the remixer audience")
	}

	{
		pending, err := getNewBlasts(pool, ctx, getNewBlastsParams{
			UserID: 202,
		})
		assert.NoError(t, err)
		assert.Len(t, pending, 1)
	}

	// 1 sends another blast to all remixers
	_, err = chatBlast(pool, ctx, 1, time.Now().UTC(), ChatBlastRPCParams{
		BlastID:  "blast_remixers_2",
		Audience: RemixerAudience,
		Message:  "new stems coming soon",
	})
	assert.NoError(t, err)

	{
		pending, err := getNewBlasts(pool, ctx, getNewBlastsParams{
			UserID: 202,
		})
		assert.NoError(t, err)
		assert.Len(t, pending, 2)
	}

	// 202 upgrades... should have 2 messages
	chatId_202_1 := trashid.ChatID(202, 1)
	err = chatCreate(pool, ctx, 202, time.Now().UTC(), ChatCreateRPCParams{
		ChatID: chatId_202_1,
		Invites: []PurpleInvite{
			{UserID: trashid.MustEncodeHashID(202), InviteCode: "earlier"},
			{UserID: trashid.MustEncodeHashID(1), InviteCode: "earlier"},
		},
	})
	assert.NoError(t, err)

	// both users have 2 messages
	{
		messages := mustGetMessagesAndReactions(t, pool, ctx, 202, chatId_202_1)
		assert.Len(t, messages, 2)
	}
	{
		messages := mustGetMessagesAndReactions(t, pool, ctx, 1, chatId_202_1)
		assert.Len(t, messages, 2)
	}

	_, err = chatBlast(pool, ctx, 1, time.Now().UTC(), ChatBlastRPCParams{
		BlastID:             "blast_remixers_3",
		Audience:            RemixerAudience,
		AudienceContentType: &trackContentType,
		AudienceContentID:   stringPointer(trashid.MustEncodeHashID(1)),
		Message:             "yall are the best",
	})
	assert.NoError(t, err)

	// both users have 3 messages
	{
		messages := mustGetMessagesAndReactions(t, pool, ctx, 202, chatId_202_1)
		assert.Len(t, messages, 3)
	}
	{
		messages := mustGetMessagesAndReactions(t, pool, ctx, 1, chatId_202_1)
		assert.Len(t, messages, 3)
	}

	{
		blastChatId := "remixer_audience:track:" + trashid.MustEncodeHashID(1)
		chat, err := getUserChat(pool, ctx, chatMembershipParams{
			UserID: 1,
			ChatID: blastChatId,
		})
		assert.NoError(t, err)
		assert.Equal(t, blastChatId, chat.ChatID)
	}

	{
		chat, err := getUserChat(pool, ctx, chatMembershipParams{
			UserID: 1,
			ChatID: "remixer_audience",
		})
		assert.NoError(t, err)
		assert.Equal(t, "remixer_audience", chat.ChatID)
	}

}

func TestChatBlastPurchasers(t *testing.T) {
	pool := database.CreateTestDatabase(t, "test_comms")
	defer pool.Close()
	database.Seed(pool, database.FixtureMap{
		"users": {
			{"user_id": 1, "wallet": "wallet1", "handle": "user1"},
			{"user_id": 203, "wallet": "wallet203", "handle": "user203"},
		},
		"tracks": {
			{
				"track_id": 1,
				"owner_id": 1,
			},
		},
		"usdc_purchases": {
			{
				"buyer_user_id":  203,
				"seller_user_id": 1,
				"content_type":   "track",
				"content_id":     1,
				"amount":         5990000, // 5.99USDC in micro-units
				"signature":      "purchase_sig_123",
				"slot":           101,
			},
		},
	})

	ctx := context.Background()

	_, err := chatBlast(pool, ctx, 1, time.Now().UTC(), ChatBlastRPCParams{
		BlastID:  "blast_customers_1",
		Audience: CustomerAudience,
		Message:  "thank you for yr purchase",
	})
	assert.NoError(t, err)

	{
		pending, err := getNewBlasts(pool, ctx, getNewBlastsParams{
			UserID: 203,
		})
		assert.NoError(t, err)
		assert.Len(t, pending, 1)
	}

	{
		chat, err := getUserChat(pool, ctx, chatMembershipParams{
			UserID: 1,
			ChatID: "customer_audience",
		})
		assert.NoError(t, err)
		assert.Equal(t, "customer_audience", chat.ChatID)
	}

	// no blasts for a specific track customer yet... so this is a not found error
	{
		_, err := getUserChat(pool, ctx, chatMembershipParams{
			UserID: 1,
			ChatID: "customer_audience:track:1",
		})
		assert.Error(t, err)
	}
}

func TestChatBlastCoinHolders(t *testing.T) {
	pool := database.CreateTestDatabase(t, "test_comms")
	defer pool.Close()
	database.Seed(pool, database.FixtureMap{
		"users": {
			{"user_id": 1, "wallet": "wallet1", "handle": "user1"},
			{"user_id": 204, "wallet": "wallet204", "handle": "user204"},
			{"user_id": 205, "wallet": "wallet205", "handle": "user205"},
			{"user_id": 206, "wallet": "wallet206", "handle": "user206"},
		},
		"artist_coins": {
			{
				"user_id":  1,
				"ticker":   "$ARTIST1",
				"mint":     "mint123",
				"decimals": 8,
			},
		},
		"sol_claimable_accounts": {
			{
				"signature":        "sig1",
				"account":          "account204",
				"ethereum_address": "wallet204",
				"mint":             "mint123",
			},
			{
				"signature":        "sig2",
				"account":          "account205",
				"ethereum_address": "wallet205",
				"mint":             "mint123",
			},
			{
				"signature":        "sig3",
				"account":          "account206",
				"ethereum_address": "wallet206",
				"mint":             "mint123",
			},
		},
	})

	ctx := context.Background()

	_, err := pool.Exec(ctx, `
	insert into sol_token_account_balance_changes
	(signature, mint, owner, account, change, balance, slot, created_at, block_timestamp)
	values
	-- user 204: positive balance before blast
	('tx1', 'mint123', 'wallet204', 'account204', 1000, 1000, 10001, $1, $1),
	('tx2', 'mint123', 'wallet206', 'account206', 500, 500, 10003, $1, $1);
	`, time.Now().UTC())
	assert.NoError(t, err)

	_, err = pool.Exec(ctx, `
	insert into sol_token_account_balance_changes
	(signature, mint, owner, account, change, balance, slot, created_at, block_timestamp)
	values
	-- user 206: had positive balance, then sold to zero before blast
	('tx3', 'mint123', 'wallet206', 'account206', -500, 0, 10004, $1, $1);
	`, time.Now().UTC())
	assert.NoError(t, err)

	// 1 sends blast to coin holders
	_, err = chatBlast(pool, ctx, 1, time.Now().UTC(), ChatBlastRPCParams{
		BlastID:  "blast_coin_holders_1",
		Audience: CoinHolderAudience,
		Message:  "thanks for holding my coin",
	})
	assert.NoError(t, err)

	// Only user 204 should have a pending blast (has positive balance)
	{
		pending, err := getNewBlasts(pool, ctx, getNewBlastsParams{
			UserID: 204,
		})
		assert.NoError(t, err)
		assert.Len(t, pending, 1)
	}

	// User 205 should have no pending blast (zero balance)
	{
		pending, err := getNewBlasts(pool, ctx, getNewBlastsParams{
			UserID: 205,
		})
		assert.NoError(t, err)
		assert.Len(t, pending, 0)
	}

	// User 206 should have no pending blast (sold before blast)
	{
		pending, err := getNewBlasts(pool, ctx, getNewBlastsParams{
			UserID: 206,
		})
		assert.NoError(t, err)
		assert.Len(t, pending, 0)
	}

	// 204 upgrades to real DM
	chatId_204_1 := trashid.ChatID(204, 1)
	err = chatCreate(pool, ctx, 204, time.Now().UTC(), ChatCreateRPCParams{
		ChatID: chatId_204_1,
		Invites: []PurpleInvite{
			{UserID: trashid.MustEncodeHashID(204), InviteCode: "earlier"},
			{UserID: trashid.MustEncodeHashID(1), InviteCode: "earlier"},
		},
	})
	assert.NoError(t, err)

	// Both users should have 1 message
	{
		messages := mustGetMessagesAndReactions(t, pool, ctx, 204, chatId_204_1)
		assert.Len(t, messages, 1)
		assert.Equal(t, "thanks for holding my coin", messages[0].Ciphertext)
	}
	{
		messages := mustGetMessagesAndReactions(t, pool, ctx, 1, chatId_204_1)
		assert.Len(t, messages, 1)
	}

	// 204 should have no pending blast after upgrade
	{
		pending, err := getNewBlasts(pool, ctx, getNewBlastsParams{
			UserID: 204,
		})
		assert.NoError(t, err)
		assert.Len(t, pending, 0)
	}

	// Test that new balance changes after blast don't affect existing blast
	_, err = pool.Exec(ctx, `
	insert into sol_token_account_balance_changes
	(signature, mint, owner, account, change, balance, slot, created_at, block_timestamp)
	values
	-- user 205 gets tokens AFTER the blast
	('tx5', 'mint123', 'wallet205', 'account205', 2000, 2000, 10005, $1, $1);
	`, time.Now().UTC())
	assert.NoError(t, err)

	// User 205 still should have no pending blast (balance change was after blast)
	{
		pending, err := getNewBlasts(pool, ctx, getNewBlastsParams{
			UserID: 205,
		})
		assert.NoError(t, err)
		assert.Len(t, pending, 0)
	}

	// Send another blast - now 205 should be included
	_, err = chatBlast(pool, ctx, 1, time.Now().UTC(), ChatBlastRPCParams{
		BlastID:  "blast_coin_holders_2",
		Audience: CoinHolderAudience,
		Message:  "welcome new holders",
	})
	assert.NoError(t, err)

	// Now user 205 should have a pending blast
	{
		pending, err := getNewBlasts(pool, ctx, getNewBlastsParams{
			UserID: 205,
		})
		assert.NoError(t, err)
		assert.Len(t, pending, 1)
	}

	// User 204 should have the new blast added to existing chat
	{
		messages := mustGetMessagesAndReactions(t, pool, ctx, 204, chatId_204_1)
		assert.Len(t, messages, 2)
		assert.Equal(t, "welcome new holders", messages[0].Ciphertext)
		assert.Equal(t, "thanks for holding my coin", messages[1].Ciphertext)
	}

	// Test blast chat view for sender
	{
		chat, err := getUserChat(pool, ctx, chatMembershipParams{
			UserID: 1,
			ChatID: "coin_holder_audience",
		})
		assert.NoError(t, err)
		assert.Equal(t, "coin_holder_audience", chat.ChatID)
	}
}

func stringPointer(val string) *string {
	return &val
}
