package api

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"api.audius.co/database"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Exercises the comment notification triggers ported from apps'
// src/tasks/entity_manager/entities/comment.py:
//
//   handle_comment_notification.sql  → comment
//   handle_comment_mention.sql       → comment_mention
//   handle_comment_thread.sql        → comment_thread
//   handle_comment_reaction.sql      → comment_reaction
//   handle_fan_club_text_post.sql    → fan_club_text_post

func TestCommentNotification_NotifiesTrackOwner(t *testing.T) {
	app := emptyTestApp(t)
	ctx := context.Background()
	require.NotNil(t, app.writePool, "test requires write pool")

	const (
		ownerId   = 8101
		fanId     = 8102
		trackId   = 8201
		commentId = 8301
	)
	now := time.Now().UTC()
	database.Seed(app.pool.Replicas[0], database.FixtureMap{
		"blocks": {{"blockhash": "cn-blk-100", "parenthash": nil, "number": 100}},
		"users": {
			{"user_id": ownerId, "handle": "cn_owner"},
			{"user_id": fanId, "handle": "cn_fan"},
		},
		"tracks": {{
			"track_id":   trackId,
			"owner_id":   ownerId,
			"title":      "Track A",
			"created_at": now,
			"updated_at": now,
		}},
	})

	// Fan leaves a top-level comment on owner's track.
	_, err := app.writePool.Exec(ctx, `
		INSERT INTO comments (
			comment_id, user_id, entity_id, entity_type,
			text, is_delete, is_visible, is_edited,
			created_at, updated_at,
			txhash, blockhash, blocknumber
		) VALUES ($1, $2, $3, 'Track', 'nice track', false, true, false,
			$4, $4, 'tx-cn-1', 'cn-blk-100', 100)
	`, commentId, fanId, trackId, now)
	require.NoError(t, err)

	type row struct {
		UserIDs []int32
		GroupID string
		Data    []byte
	}
	var r row
	require.NoError(t, app.writePool.QueryRow(ctx, `
		SELECT user_ids, group_id, data
		FROM notification
		WHERE type = 'comment'
		  AND group_id = $1
	`, "comment:8201:type:Track").Scan(&r.UserIDs, &r.GroupID, &r.Data))
	assert.Equal(t, []int32{ownerId}, r.UserIDs)
	var data map[string]any
	require.NoError(t, json.Unmarshal(r.Data, &data))
	assert.Equal(t, "Track", data["type"])
	assert.EqualValues(t, trackId, data["entity_id"])
	assert.EqualValues(t, fanId, data["comment_user_id"])
	assert.EqualValues(t, commentId, data["comment_id"])
}

func TestCommentNotification_SkipsSelfComment(t *testing.T) {
	app := emptyTestApp(t)
	ctx := context.Background()

	const (
		ownerId   = 8401
		trackId   = 8501
		commentId = 8601
	)
	now := time.Now().UTC()
	database.Seed(app.pool.Replicas[0], database.FixtureMap{
		"blocks": {{"blockhash": "cn-blk-150", "parenthash": nil, "number": 150}},
		"users":  {{"user_id": ownerId, "handle": "cn_self"}},
		"tracks": {{
			"track_id": trackId, "owner_id": ownerId, "title": "Own",
			"created_at": now, "updated_at": now,
		}},
	})
	_, err := app.writePool.Exec(ctx, `
		INSERT INTO comments (comment_id, user_id, entity_id, entity_type,
			text, is_delete, is_visible, is_edited, created_at, updated_at,
			txhash, blockhash, blocknumber)
		VALUES ($1, $2, $3, 'Track', 'self', false, true, false,
			$4, $4, 'tx-cn-2', 'cn-blk-150', 150)
	`, commentId, ownerId, trackId, now)
	require.NoError(t, err)

	var n int
	require.NoError(t, app.writePool.QueryRow(ctx, `
		SELECT count(*) FROM notification WHERE type = 'comment'
	`).Scan(&n))
	assert.Equal(t, 0, n, "no self-comment notification")
}

func TestCommentNotification_SkipsReply(t *testing.T) {
	app := emptyTestApp(t)
	ctx := context.Background()

	const (
		ownerId         = 8701
		fanId           = 8702
		trackId         = 8801
		parentCommentId = 8901
		replyCommentId  = 8902
	)
	now := time.Now().UTC()
	database.Seed(app.pool.Replicas[0], database.FixtureMap{
		"blocks": {
			{"blockhash": "cn-blk-200", "parenthash": nil, "number": 200},
			{"blockhash": "cn-blk-201", "parenthash": "cn-blk-200", "number": 201},
		},
		"users": {
			{"user_id": ownerId, "handle": "cnr_owner"},
			{"user_id": fanId, "handle": "cnr_fan"},
		},
		"tracks": {{
			"track_id": trackId, "owner_id": ownerId, "title": "T",
			"created_at": now, "updated_at": now,
		}},
		"comments": {{
			"comment_id": parentCommentId, "user_id": fanId,
			"entity_id": trackId, "entity_type": "Track",
			"text": "first", "is_delete": false, "is_visible": true,
			"created_at": now, "updated_at": now,
			"txhash": "tx-parent", "blockhash": "cn-blk-200", "blocknumber": 200,
		}},
	})

	// The reply must be inserted alongside its comment_threads row in the
	// same transaction so the deferred trigger correctly skips it.
	tx, err := app.writePool.Begin(ctx)
	require.NoError(t, err)
	_, err = tx.Exec(ctx, `
		INSERT INTO comments (comment_id, user_id, entity_id, entity_type,
			text, is_delete, is_visible, is_edited, created_at, updated_at,
			txhash, blockhash, blocknumber)
		VALUES ($1, $2, $3, 'Track', 'reply', false, true, false,
			$4, $4, 'tx-reply', 'cn-blk-201', 201)
	`, replyCommentId, fanId, trackId, now)
	require.NoError(t, err)
	_, err = tx.Exec(ctx, `
		INSERT INTO comment_threads (parent_comment_id, comment_id)
		VALUES ($1, $2)
	`, parentCommentId, replyCommentId)
	require.NoError(t, err)
	require.NoError(t, tx.Commit(ctx))

	// Only the parent comment should produce a `comment` notification.
	var count int
	require.NoError(t, app.writePool.QueryRow(ctx, `
		SELECT count(*) FROM notification
		WHERE type = 'comment' AND group_id = 'comment:8801:type:Track'
	`).Scan(&count))
	assert.Equal(t, 1, count, "reply should not produce a second comment notification")
}

func TestCommentMention_NotifiesMentionedUser(t *testing.T) {
	app := emptyTestApp(t)
	ctx := context.Background()

	const (
		ownerId   = 9001
		fanId     = 9002
		mentionId = 9003
		trackId   = 9101
		commentId = 9201
	)
	now := time.Now().UTC()
	database.Seed(app.pool.Replicas[0], database.FixtureMap{
		"blocks": {{"blockhash": "cm-blk-100", "parenthash": nil, "number": 100}},
		"users": {
			{"user_id": ownerId, "handle": "cm_owner"},
			{"user_id": fanId, "handle": "cm_fan"},
			{"user_id": mentionId, "handle": "cm_mentioned"},
		},
		"tracks": {{
			"track_id": trackId, "owner_id": ownerId, "title": "T",
			"created_at": now, "updated_at": now,
		}},
		"comments": {{
			"comment_id": commentId, "user_id": fanId,
			"entity_id": trackId, "entity_type": "Track",
			"text": "@cm_mentioned check this out",
			"is_delete": false, "is_visible": true,
			"created_at": now, "updated_at": now,
			"txhash": "tx-cm", "blockhash": "cm-blk-100", "blocknumber": 100,
		}},
	})

	_, err := app.writePool.Exec(ctx, `
		INSERT INTO comment_mentions (comment_id, user_id, is_delete,
			created_at, updated_at, txhash, blockhash, blocknumber)
		VALUES ($1, $2, false, $3, $3, 'tx-cm', 'cm-blk-100', 100)
	`, commentId, mentionId, now)
	require.NoError(t, err)

	type row struct {
		UserIDs []int32
		GroupID string
		Data    []byte
	}
	var r row
	require.NoError(t, app.writePool.QueryRow(ctx, `
		SELECT user_ids, group_id, data
		FROM notification
		WHERE type = 'comment_mention'
		  AND group_id = $1
	`, "comment_mention:9201").Scan(&r.UserIDs, &r.GroupID, &r.Data))
	assert.Equal(t, []int32{mentionId}, r.UserIDs)
	var data map[string]any
	require.NoError(t, json.Unmarshal(r.Data, &data))
	assert.EqualValues(t, fanId, data["comment_user_id"])
	assert.EqualValues(t, ownerId, data["entity_user_id"])
	assert.EqualValues(t, trackId, data["entity_id"])
}

func TestCommentMention_SkipsWhenMentionedMutedCommenter(t *testing.T) {
	app := emptyTestApp(t)
	ctx := context.Background()

	const (
		ownerId   = 9301
		fanId     = 9302
		mentionId = 9303
		trackId   = 9401
		commentId = 9501
	)
	now := time.Now().UTC()
	database.Seed(app.pool.Replicas[0], database.FixtureMap{
		"blocks": {{"blockhash": "cm-blk-300", "parenthash": nil, "number": 300}},
		"users": {
			{"user_id": ownerId, "handle": "cmm_owner"},
			{"user_id": fanId, "handle": "cmm_fan"},
			{"user_id": mentionId, "handle": "cmm_mentioned"},
		},
		"tracks": {{
			"track_id": trackId, "owner_id": ownerId, "title": "T",
			"created_at": now, "updated_at": now,
		}},
		"comments": {{
			"comment_id": commentId, "user_id": fanId,
			"entity_id": trackId, "entity_type": "Track",
			"text": "@cmm_mentioned hi", "is_delete": false, "is_visible": true,
			"created_at": now, "updated_at": now,
			"txhash": "tx-cmm", "blockhash": "cm-blk-300", "blocknumber": 300,
		}},
		// Mentioned user has muted the commenter.
		"muted_users": {{
			"muted_user_id": fanId, "user_id": mentionId,
			"is_delete": false,
			"created_at": now, "updated_at": now,
			"txhash": "seed-mute", "blockhash": "cm-blk-300", "blocknumber": 300,
		}},
	})

	_, err := app.writePool.Exec(ctx, `
		INSERT INTO comment_mentions (comment_id, user_id, is_delete,
			created_at, updated_at, txhash, blockhash, blocknumber)
		VALUES ($1, $2, false, $3, $3, 'tx-cmm', 'cm-blk-300', 300)
	`, commentId, mentionId, now)
	require.NoError(t, err)

	var n int
	require.NoError(t, app.writePool.QueryRow(ctx, `
		SELECT count(*) FROM notification WHERE type = 'comment_mention'
	`).Scan(&n))
	assert.Equal(t, 0, n, "mention notification suppressed by muted_users")
}

func TestCommentThread_NotifiesParentAuthor(t *testing.T) {
	app := emptyTestApp(t)
	ctx := context.Background()

	const (
		ownerId         = 9601
		parentUserId    = 9602
		replyUserId     = 9603
		trackId         = 9701
		parentCommentId = 9801
		replyCommentId  = 9802
	)
	now := time.Now().UTC()
	database.Seed(app.pool.Replicas[0], database.FixtureMap{
		"blocks": {
			{"blockhash": "ct-blk-300", "parenthash": nil, "number": 300},
			{"blockhash": "ct-blk-301", "parenthash": "ct-blk-300", "number": 301},
		},
		"users": {
			{"user_id": ownerId, "handle": "ct_owner"},
			{"user_id": parentUserId, "handle": "ct_parent"},
			{"user_id": replyUserId, "handle": "ct_reply"},
		},
		"tracks": {{
			"track_id": trackId, "owner_id": ownerId, "title": "T",
			"created_at": now, "updated_at": now,
		}},
		"comments": {
			{
				"comment_id": parentCommentId, "user_id": parentUserId,
				"entity_id": trackId, "entity_type": "Track",
				"text": "parent", "is_delete": false, "is_visible": true,
				"created_at": now, "updated_at": now,
				"txhash": "tx-ct-p", "blockhash": "ct-blk-300", "blocknumber": 300,
			},
			{
				"comment_id": replyCommentId, "user_id": replyUserId,
				"entity_id": trackId, "entity_type": "Track",
				"text": "reply", "is_delete": false, "is_visible": true,
				"created_at": now, "updated_at": now,
				"txhash": "tx-ct-r", "blockhash": "ct-blk-301", "blocknumber": 301,
			},
		},
	})

	_, err := app.writePool.Exec(ctx, `
		INSERT INTO comment_threads (parent_comment_id, comment_id)
		VALUES ($1, $2)
	`, parentCommentId, replyCommentId)
	require.NoError(t, err)

	type row struct {
		UserIDs []int32
		Data    []byte
	}
	var r row
	require.NoError(t, app.writePool.QueryRow(ctx, `
		SELECT user_ids, data
		FROM notification
		WHERE type = 'comment_thread'
		  AND group_id = $1
		  AND specifier = $2
	`, "comment_thread:9801", "9802").Scan(&r.UserIDs, &r.Data))
	assert.Equal(t, []int32{parentUserId}, r.UserIDs)
	var data map[string]any
	require.NoError(t, json.Unmarshal(r.Data, &data))
	assert.EqualValues(t, replyUserId, data["comment_user_id"])
	assert.EqualValues(t, replyCommentId, data["comment_id"])
}

func TestCommentReaction_NotifiesCommentAuthor(t *testing.T) {
	app := emptyTestApp(t)
	ctx := context.Background()

	const (
		ownerId      = 10001
		authorId     = 10002
		reacterId    = 10003
		trackId      = 10101
		commentId    = 10201
	)
	now := time.Now().UTC()
	database.Seed(app.pool.Replicas[0], database.FixtureMap{
		"blocks": {{"blockhash": "cr-blk-100", "parenthash": nil, "number": 100}},
		"users": {
			{"user_id": ownerId, "handle": "cr_owner"},
			{"user_id": authorId, "handle": "cr_author"},
			{"user_id": reacterId, "handle": "cr_reacter"},
		},
		"tracks": {{
			"track_id": trackId, "owner_id": ownerId, "title": "T",
			"created_at": now, "updated_at": now,
		}},
		"comments": {{
			"comment_id": commentId, "user_id": authorId,
			"entity_id": trackId, "entity_type": "Track",
			"text": "nice", "is_delete": false, "is_visible": true,
			"created_at": now, "updated_at": now,
			"txhash": "tx-cr-c", "blockhash": "cr-blk-100", "blocknumber": 100,
		}},
	})

	_, err := app.writePool.Exec(ctx, `
		INSERT INTO comment_reactions (comment_id, user_id, is_delete,
			created_at, updated_at, txhash, blockhash, blocknumber)
		VALUES ($1, $2, false, $3, $3, 'tx-cr-r', 'cr-blk-100', 100)
	`, commentId, reacterId, now)
	require.NoError(t, err)

	type row struct {
		UserIDs   []int32
		GroupID   string
		Specifier string
		Data      []byte
	}
	var r row
	require.NoError(t, app.writePool.QueryRow(ctx, `
		SELECT user_ids, group_id, specifier, data
		FROM notification
		WHERE type = 'comment_reaction'
		  AND group_id = $1
	`, "comment_reaction:10201").Scan(&r.UserIDs, &r.GroupID, &r.Specifier, &r.Data))
	assert.Equal(t, []int32{authorId}, r.UserIDs)
	assert.Equal(t, "10003", r.Specifier, "specifier is the reacter user_id")
	var data map[string]any
	require.NoError(t, json.Unmarshal(r.Data, &data))
	assert.EqualValues(t, reacterId, data["reacter_user_id"])
	assert.EqualValues(t, commentId, data["comment_id"])
	assert.EqualValues(t, ownerId, data["entity_user_id"])
}

func TestFanClubTextPost_FansOutToFollowersAndCoinHolders(t *testing.T) {
	app := emptyTestApp(t)
	ctx := context.Background()

	const (
		artistId       = 11001
		followerId     = 11002
		coinHolderId   = 11003
		bothId         = 11004 // both follower and coin holder — should still get only one row
		strangerId     = 11005 // neither — should not get notified
		commentId      = 11101
	)
	now := time.Now().UTC()
	mint := "MintAlpha111"

	database.Seed(app.pool.Replicas[0], database.FixtureMap{
		"blocks": {{"blockhash": "fc-blk-100", "parenthash": nil, "number": 100}},
		"users": {
			{"user_id": artistId, "handle": "fc_artist"},
			{"user_id": followerId, "handle": "fc_follower"},
			{"user_id": coinHolderId, "handle": "fc_coin"},
			{"user_id": bothId, "handle": "fc_both"},
			{"user_id": strangerId, "handle": "fc_stranger"},
		},
		"follows": {
			{"follower_user_id": followerId, "followee_user_id": artistId,
				"is_current": true, "is_delete": false, "created_at": now, "blocknumber": 100, "blockhash": "fc-blk-100"},
			{"follower_user_id": bothId, "followee_user_id": artistId,
				"is_current": true, "is_delete": false, "created_at": now, "blocknumber": 100, "blockhash": "fc-blk-100"},
		},
		"artist_coins": {{
			"mint": mint, "ticker": "FCAR", "user_id": artistId,
			"decimals": 6, "name": "Fc Artist Coin",
		}},
		"sol_user_balances": {
			{"user_id": coinHolderId, "mint": mint, "balance": 100, "created_at": now, "updated_at": now},
			{"user_id": bothId, "mint": mint, "balance": 50, "created_at": now, "updated_at": now},
		},
	})

	// Artist posts a top-level text update on their fan club.
	_, err := app.writePool.Exec(ctx, `
		INSERT INTO comments (comment_id, user_id, entity_id, entity_type,
			text, is_delete, is_visible, is_edited, is_members_only,
			created_at, updated_at, txhash, blockhash, blocknumber)
		VALUES ($1, $2, $2, 'FanClub', 'studio update!', false, true, false, true,
			$3, $3, 'tx-fc', 'fc-blk-100', 100)
	`, commentId, artistId, now)
	require.NoError(t, err)

	// followerId, coinHolderId, and bothId should all be notified once. artistId never.
	rows, err := app.writePool.Query(ctx, `
		SELECT user_ids, specifier
		  FROM notification
		 WHERE type = 'fan_club_text_post'
		   AND group_id = $1
		 ORDER BY specifier
	`, "fan_club_text_post:11101:user:11001")
	require.NoError(t, err)
	defer rows.Close()

	recipients := map[int32]bool{}
	for rows.Next() {
		var userIDs []int32
		var specifier string
		require.NoError(t, rows.Scan(&userIDs, &specifier))
		require.Len(t, userIDs, 1)
		recipients[userIDs[0]] = true
	}
	require.NoError(t, rows.Err())

	assert.True(t, recipients[followerId], "follower must be notified")
	assert.True(t, recipients[coinHolderId], "coin holder must be notified")
	assert.True(t, recipients[bothId], "user who is both follower and coin holder gets exactly one notification (UNION)")
	assert.False(t, recipients[strangerId], "stranger must not be notified")
	assert.False(t, recipients[artistId], "artist (post author) must not be notified")
	assert.Len(t, recipients, 3, "exactly 3 unique recipients")
}

func TestFanClubTextPost_SkipsFanComments(t *testing.T) {
	app := emptyTestApp(t)
	ctx := context.Background()

	const (
		artistId  = 11201
		fanId     = 11202
		commentId = 11301
	)
	now := time.Now().UTC()
	database.Seed(app.pool.Replicas[0], database.FixtureMap{
		"blocks": {{"blockhash": "fc-blk-200", "parenthash": nil, "number": 200}},
		"users": {
			{"user_id": artistId, "handle": "fc_artist2"},
			{"user_id": fanId, "handle": "fc_fan"},
		},
	})

	// A fan (not the artist) posts a top-level comment on the artist's
	// fan club. Should NOT fan out — fan_club_text_post is only for the
	// artist's own posts. The `comment` notification (to the artist)
	// fires instead, but we're only checking fan_club_text_post here.
	_, err := app.writePool.Exec(ctx, `
		INSERT INTO comments (comment_id, user_id, entity_id, entity_type,
			text, is_delete, is_visible, is_edited, is_members_only,
			created_at, updated_at, txhash, blockhash, blocknumber)
		VALUES ($1, $2, $3, 'FanClub', 'love your work', false, true, false, true,
			$4, $4, 'tx-fc-2', 'fc-blk-200', 200)
	`, commentId, fanId, artistId, now)
	require.NoError(t, err)

	var n int
	require.NoError(t, app.writePool.QueryRow(ctx, `
		SELECT count(*) FROM notification WHERE type = 'fan_club_text_post'
	`).Scan(&n))
	assert.Equal(t, 0, n, "fan posts do not trigger fan_club_text_post")
}
