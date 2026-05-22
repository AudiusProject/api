package api

import (
	"context"
	"testing"
	"time"

	"api.audius.co/database"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRemixContestUpdate_NotifiesEventSubscribers exercises the
// handle_comment_remix_contest_update trigger. When the contest host posts
// a top-level (non-reply) comment on their own remix-contest event, every
// active event subscriber (excluding the host) should receive one
// `remix_contest_update` notification row.
func TestRemixContestUpdate_NotifiesEventSubscribers(t *testing.T) {
	app := emptyTestApp(t)
	ctx := context.Background()
	require.NotNil(t, app.writePool, "test requires write pool")

	hostId := 7101
	subA := 7102
	subB := 7103
	parentTrackId := 7201
	eventId := 7301
	commentId := 7401

	now := time.Now().UTC()
	fixtures := database.FixtureMap{
		// comments has an FK on blocknumber → blocks.number; the comment we
		// insert later uses blocknumber=100, so seed it.
		"blocks": []map[string]any{
			{"blockhash": "rcu-blk-100", "parenthash": nil, "number": 100},
		},
		"users": []map[string]any{
			{"user_id": hostId, "handle": "rc_host"},
			{"user_id": subA, "handle": "rc_subA"},
			{"user_id": subB, "handle": "rc_subB"},
		},
		"tracks": []map[string]any{
			{
				"track_id":   parentTrackId,
				"owner_id":   hostId,
				"title":      "Parent Track",
				"created_at": now,
				"updated_at": now,
			},
		},
		"events": []map[string]any{
			{
				"event_id":   eventId,
				"event_type": "remix_contest",
				"entity_id":  parentTrackId,
				"user_id":    hostId,
				"created_at": now,
				"end_date":   now.Add(7 * 24 * time.Hour),
			},
		},
		"subscriptions": []map[string]any{
			// Both subA and subB subscribe to the event. Host also "subscribes"
			// to their own event (which should still be excluded by the trigger).
			{
				"subscriber_id": subA,
				"user_id":       eventId,
				"entity_type":   "Event",
				"entity_id":     eventId,
				"is_current":    true,
				"is_delete":     false,
				"created_at":    now,
				"txhash":        "seed-subA",
			},
			{
				"subscriber_id": subB,
				"user_id":       eventId,
				"entity_type":   "Event",
				"entity_id":     eventId,
				"is_current":    true,
				"is_delete":     false,
				"created_at":    now,
				"txhash":        "seed-subB",
			},
			{
				"subscriber_id": hostId,
				"user_id":       eventId,
				"entity_type":   "Event",
				"entity_id":     eventId,
				"is_current":    true,
				"is_delete":     false,
				"created_at":    now,
				"txhash":        "seed-subhost",
			},
		},
	}
	database.Seed(app.pool.Replicas[0], fixtures)

	// Host posts a TOP-LEVEL comment on their own event. Single auto-commit
	// statement → deferred trigger fires at the commit → comment_threads is
	// empty for this comment_id → treated as top-level → fans out.
	_, err := app.writePool.Exec(ctx, `
		INSERT INTO comments (
			comment_id, user_id, entity_id, entity_type,
			text, is_delete, is_visible, is_edited,
			created_at, updated_at,
			txhash, blockhash, blocknumber
		) VALUES (
			$1, $2, $3, 'Event',
			'Round 1 is live!', false, true, false,
			$4, $4,
			'tx-rcu-1', 'blk-rcu-1', 100
		)
	`, commentId, hostId, eventId, now)
	require.NoError(t, err)

	// Each non-host subscriber should now have exactly one
	// remix_contest_update notification.
	type notifRow struct {
		Specifier    string
		GroupId      string
		UserIds      []int32
		EventId      int
		EntityId     int
		EntityUserId int
		CommentId    int
	}
	rows, err := app.writePool.Query(ctx, `
		SELECT specifier, group_id, user_ids,
		       (data->>'event_id')::int,
		       (data->>'entity_id')::int,
		       (data->>'entity_user_id')::int,
		       (data->>'comment_id')::int
		  FROM notification
		 WHERE type = 'remix_contest_update'
		   AND group_id = $1
		 ORDER BY specifier ASC
	`, "remix_contest_update:7401:event:7301")
	require.NoError(t, err)
	defer rows.Close()

	var got []notifRow
	for rows.Next() {
		var r notifRow
		require.NoError(t, rows.Scan(&r.Specifier, &r.GroupId, &r.UserIds,
			&r.EventId, &r.EntityId, &r.EntityUserId, &r.CommentId))
		got = append(got, r)
	}
	require.NoError(t, rows.Err())

	require.Len(t, got, 2, "expected exactly one notification per non-host subscriber")

	recipients := map[int32]bool{}
	for _, r := range got {
		assert.Equal(t, eventId, r.EventId)
		assert.Equal(t, parentTrackId, r.EntityId, "entity_id = the contest's parent track")
		assert.Equal(t, hostId, r.EntityUserId, "entity_user_id = the event host")
		assert.Equal(t, commentId, r.CommentId)
		require.Len(t, r.UserIds, 1)
		recipients[r.UserIds[0]] = true
	}
	assert.True(t, recipients[int32(subA)], "subA must receive the notification")
	assert.True(t, recipients[int32(subB)], "subB must receive the notification")
	assert.False(t, recipients[int32(hostId)], "host must NOT receive a notification for their own post")
}

// TestRemixContestUpdate_SkipsReplies verifies that a HOST reply (a
// comment with a comment_threads row inserted in the same transaction)
// does NOT trigger remix_contest_update. Only top-level posts do.
func TestRemixContestUpdate_SkipsReplies(t *testing.T) {
	app := emptyTestApp(t)
	ctx := context.Background()
	require.NotNil(t, app.writePool, "test requires write pool")

	hostId := 7501
	subId := 7502
	parentTrackId := 7601
	eventId := 7701
	parentCommentId := 7801
	replyCommentId := 7802

	now := time.Now().UTC()
	fixtures := database.FixtureMap{
		// comments has an FK on blocknumber → blocks.number; both the seeded
		// parent comment (blocknumber=99) and the reply we insert later
		// (blocknumber=100) need their blocks rows.
		"blocks": []map[string]any{
			{"blockhash": "rcu-blk-99", "parenthash": nil, "number": 99},
			{"blockhash": "rcu-blk-100", "parenthash": "rcu-blk-99", "number": 100},
		},
		"users": []map[string]any{
			{"user_id": hostId, "handle": "rcr_host"},
			{"user_id": subId, "handle": "rcr_sub"},
		},
		"tracks": []map[string]any{
			{"track_id": parentTrackId, "owner_id": hostId, "title": "Parent",
				"created_at": now, "updated_at": now},
		},
		"events": []map[string]any{
			{"event_id": eventId, "event_type": "remix_contest",
				"entity_id": parentTrackId, "user_id": hostId,
				"created_at": now, "end_date": now.Add(7 * 24 * time.Hour)},
		},
		"subscriptions": []map[string]any{
			{"subscriber_id": subId, "user_id": eventId, "entity_type": "Event",
				"entity_id": eventId, "is_current": true, "is_delete": false,
				"created_at": now, "txhash": "seed-sub-r"},
		},
		// Seed an existing top-level host comment to be the reply's parent.
		"comments": []map[string]any{
			{"comment_id": parentCommentId, "user_id": hostId,
				"entity_id": eventId, "entity_type": "Event",
				"text": "opener", "is_delete": false, "is_visible": true,
				"created_at": now, "updated_at": now,
				"txhash": "tx-parent", "blockhash": "rcu-blk-99", "blocknumber": 99},
		},
	}
	database.Seed(app.pool.Replicas[0], fixtures)

	// We need the comment INSERT + comment_threads INSERT in the same tx so
	// the deferred trigger sees the thread row at commit time.
	tx, err := app.writePool.Begin(ctx)
	require.NoError(t, err)
	_, err = tx.Exec(ctx, `
		INSERT INTO comments (
			comment_id, user_id, entity_id, entity_type,
			text, is_delete, is_visible, is_edited,
			created_at, updated_at,
			txhash, blockhash, blocknumber
		) VALUES (
			$1, $2, $3, 'Event',
			'my reply', false, true, false,
			$4, $4,
			'tx-reply', 'blk-reply', 100
		)
	`, replyCommentId, hostId, eventId, now)
	require.NoError(t, err)
	_, err = tx.Exec(ctx, `
		INSERT INTO comment_threads (parent_comment_id, comment_id)
		VALUES ($1, $2)
	`, parentCommentId, replyCommentId)
	require.NoError(t, err)
	require.NoError(t, tx.Commit(ctx))

	// No remix_contest_update notification should exist for the reply.
	var n int
	err = app.writePool.QueryRow(ctx, `
		SELECT count(*) FROM notification
		 WHERE type = 'remix_contest_update'
		   AND group_id = $1
	`, "remix_contest_update:7802:event:7701").Scan(&n)
	require.NoError(t, err)
	assert.Equal(t, 0, n, "host replies must NOT trigger remix_contest_update")
}

// TestRemixContestUpdate_SkipsNonHostComments verifies that a NON-host
// commenter on the event does not trigger the notification, even if
// they're commenting top-level.
func TestRemixContestUpdate_SkipsNonHostComments(t *testing.T) {
	app := emptyTestApp(t)
	ctx := context.Background()
	require.NotNil(t, app.writePool, "test requires write pool")

	hostId := 7901
	commenterId := 7902
	subId := 7903
	parentTrackId := 7801
	eventId := 7811
	commentId := 7821

	now := time.Now().UTC()
	fixtures := database.FixtureMap{
		// comments has an FK on blocknumber → blocks.number.
		"blocks": []map[string]any{
			{"blockhash": "rcu-blk-100", "parenthash": nil, "number": 100},
		},
		"users": []map[string]any{
			{"user_id": hostId, "handle": "rcn_host"},
			{"user_id": commenterId, "handle": "rcn_commenter"},
			{"user_id": subId, "handle": "rcn_sub"},
		},
		"tracks": []map[string]any{
			{"track_id": parentTrackId, "owner_id": hostId, "title": "Parent",
				"created_at": now, "updated_at": now},
		},
		"events": []map[string]any{
			{"event_id": eventId, "event_type": "remix_contest",
				"entity_id": parentTrackId, "user_id": hostId,
				"created_at": now, "end_date": now.Add(7 * 24 * time.Hour)},
		},
		"subscriptions": []map[string]any{
			{"subscriber_id": subId, "user_id": eventId, "entity_type": "Event",
				"entity_id": eventId, "is_current": true, "is_delete": false,
				"created_at": now, "txhash": "seed-sub-nh"},
		},
	}
	database.Seed(app.pool.Replicas[0], fixtures)

	// Non-host posts a top-level comment on the event.
	_, err := app.writePool.Exec(ctx, `
		INSERT INTO comments (
			comment_id, user_id, entity_id, entity_type,
			text, is_delete, is_visible, is_edited,
			created_at, updated_at,
			txhash, blockhash, blocknumber
		) VALUES (
			$1, $2, $3, 'Event',
			'I hope I win!', false, true, false,
			$4, $4,
			'tx-nh', 'blk-nh', 100
		)
	`, commentId, commenterId, eventId, now)
	require.NoError(t, err)

	var n int
	err = app.writePool.QueryRow(ctx, `
		SELECT count(*) FROM notification WHERE type = 'remix_contest_update'
	`).Scan(&n)
	require.NoError(t, err)
	assert.Equal(t, 0, n, "non-host comments must NOT trigger remix_contest_update")
}
