package api

import (
	"context"
	"testing"
	"time"

	"api.audius.co/database"
	"api.audius.co/trashid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTrackRemixes(t *testing.T) {

	app := emptyTestApp(t)
	ownerId := 5001
	firstRemixOwnerId := 5002
	secondRemixOwnerId := 5003
	thirdRemixOwnerId := 5004
	fourthRemixOwnerId := 5005
	parentTrackId := 7001
	firstRemixTrackId := 7002
	secondRemixTrackId := 7003
	thirdRemixTrackId := 7004
	fourthRemixTrackId := 7005
	fixtures := database.FixtureMap{
		"events": []map[string]any{
			{
				"event_id":   3,
				"event_type": "remix_contest",
				"entity_id":  parentTrackId,
				"user_id":    ownerId,
				"created_at": parseTime(t, "2024-01-02"),
				"end_date":   parseTime(t, "2024-01-06"),
			},
			// Past event (should be ignored)
			{
				"event_id":   2,
				"event_type": "remix_contest",
				"entity_id":  parentTrackId,
				"user_id":    ownerId,
				"created_at": parseTime(t, "2023-01-02"),
				"end_date":   parseTime(t, "2023-01-06"),
			},
			// deleted event (should be ignored)
			{
				"event_id":   1,
				"is_deleted": true,
				"event_type": "remix_contest",
				"entity_id":  parentTrackId,
				"user_id":    ownerId,
				"created_at": parseTime(t, "2023-06-01"),
				"end_date":   parseTime(t, "2023-06-06"),
			},
		},
		"tracks": []map[string]any{
			{
				"track_id":   parentTrackId,
				"owner_id":   ownerId,
				"title":      "Parent Track",
				"created_at": parseTime(t, "2024-01-02"),
			},
			{
				"track_id":   firstRemixTrackId,
				"owner_id":   firstRemixOwnerId,
				"title":      "First Remix Track (Reposted)",
				"created_at": parseTime(t, "2024-01-03"),
			},
			{
				"track_id":   secondRemixTrackId,
				"owner_id":   secondRemixOwnerId,
				"title":      "Second Remix Track (Saved)",
				"created_at": parseTime(t, "2024-01-04"),
			},
			{
				"track_id":   thirdRemixTrackId,
				"owner_id":   thirdRemixOwnerId,
				"title":      "Third Remix Track (Too Late)",
				"created_at": parseTime(t, "2024-01-07"),
			},
			{
				"track_id":   fourthRemixTrackId,
				"owner_id":   fourthRemixOwnerId,
				"title":      "Fourth Remix Track (Too Early)",
				"created_at": parseTime(t, "2024-01-01"),
			},
		},
		"users": []map[string]any{
			{
				"user_id": ownerId,
				"handle":  "owner",
			},
			{
				"user_id": firstRemixOwnerId,
				"handle":  "first_remix_owner",
			},
			{
				"user_id": secondRemixOwnerId,
				"handle":  "second_remix_owner",
			},
			{
				"user_id": thirdRemixOwnerId,
				"handle":  "third_remix_owner",
			},
			{
				"user_id": fourthRemixOwnerId,
				"handle":  "fourth_remix_owner",
			},
		},
		"remixes": []map[string]any{
			{
				"parent_track_id": parentTrackId,
				"child_track_id":  firstRemixTrackId,
			},
			{
				"parent_track_id": parentTrackId,
				"child_track_id":  secondRemixTrackId,
			},
			{
				"parent_track_id": parentTrackId,
				"child_track_id":  thirdRemixTrackId,
			},
			{
				"parent_track_id": parentTrackId,
				"child_track_id":  fourthRemixTrackId,
			},
		},
		"aggregate_plays": []map[string]any{
			{
				"play_item_id": firstRemixTrackId,
				"count":        100,
			},
			{
				"play_item_id": secondRemixTrackId,
				"count":        200,
			},
			{
				"play_item_id": thirdRemixTrackId,
				"count":        300,
			},
			{
				"play_item_id": fourthRemixTrackId,
				"count":        400,
			},
		},
		"reposts": []map[string]any{
			{
				"repost_item_id": firstRemixTrackId,
				"user_id":        ownerId,
				"repost_type":    "track",
				"created_at":     parseTime(t, "2024-01-03"),
			},
		},
		"saves": []map[string]any{
			{
				"save_item_id": secondRemixTrackId,
				"save_type":    "track",
				"user_id":      ownerId,
				"created_at":   parseTime(t, "2024-01-04"),
			},
			{
				"save_item_id": secondRemixTrackId,
				"save_type":    "track",
				"user_id":      firstRemixOwnerId,
				"created_at":   parseTime(t, "2024-01-04"),
			},
			{
				"save_item_id": firstRemixTrackId,
				"save_type":    "track",
				"user_id":      thirdRemixOwnerId,
				"created_at":   parseTime(t, "2024-01-04"),
			},
		},
	}

	database.Seed(app.pool.Replicas[0], fixtures)

	var baseUrl = "/v1/full/tracks/" + trashid.MustEncodeHashID(parentTrackId) + "/remixes"

	// Default sort: recent, no filtering
	status, body := testGet(t, app, baseUrl)
	assert.Equal(t, 200, status)

	jsonAssert(t, body, map[string]any{
		"data.tracks.#":    4,
		"data.tracks.0.id": trashid.MustEncodeHashID(thirdRemixTrackId),
		"data.tracks.1.id": trashid.MustEncodeHashID(secondRemixTrackId),
		"data.tracks.2.id": trashid.MustEncodeHashID(firstRemixTrackId),
		"data.tracks.3.id": trashid.MustEncodeHashID(fourthRemixTrackId),
	})

	// Sort by likes
	status, body = testGet(t, app, baseUrl+"?sort_method=likes")

	// should be saves desc, then track_id desc for ties
	jsonAssert(t, body, map[string]any{
		"data.tracks.#":    4,
		"data.tracks.0.id": trashid.MustEncodeHashID(secondRemixTrackId),
		"data.tracks.1.id": trashid.MustEncodeHashID(firstRemixTrackId),
		"data.tracks.2.id": trashid.MustEncodeHashID(fourthRemixTrackId),
		"data.tracks.3.id": trashid.MustEncodeHashID(thirdRemixTrackId),
	})

	// Sort by plays
	status, body = testGet(t, app, baseUrl+"?sort_method=plays")

	// should be plays desc, then track_id desc for ties
	jsonAssert(t, body, map[string]any{
		"data.tracks.#":    4,
		"data.tracks.0.id": trashid.MustEncodeHashID(fourthRemixTrackId),
		"data.tracks.1.id": trashid.MustEncodeHashID(thirdRemixTrackId),
		"data.tracks.2.id": trashid.MustEncodeHashID(secondRemixTrackId),
		"data.tracks.3.id": trashid.MustEncodeHashID(firstRemixTrackId),
	})

	// contest entries only
	status, body = testGet(t, app, baseUrl+"?sort_method=recent&only_contest_entries=true")

	jsonAssert(t, body, map[string]any{
		"data.tracks.#":    2,
		"data.tracks.0.id": trashid.MustEncodeHashID(secondRemixTrackId),
		"data.tracks.1.id": trashid.MustEncodeHashID(firstRemixTrackId),
	})

	// cosigns only
	status, body = testGet(t, app, baseUrl+"?sort_method=recent&only_cosigns=true")

	jsonAssert(t, body, map[string]any{
		"data.tracks.#":    2,
		"data.tracks.0.id": trashid.MustEncodeHashID(secondRemixTrackId),
		"data.tracks.1.id": trashid.MustEncodeHashID(firstRemixTrackId),
	})
}

// When an artist creates a remix contest while their track is still unlisted,
// the on_event trigger skips the fan_remix_contest_started notifications because
// the track is not yet public. When the artist later flips is_unlisted to false,
// the on_track trigger must pick up the active contest and create the missing
// notifications for the contest creator's followers and the track's savers.
func TestFanRemixContestStartedOnUnlistedToPublic(t *testing.T) {
	app := emptyTestApp(t)
	ctx := context.Background()
	require.NotNil(t, app.writePool, "test requires write pool")

	ownerId := 9001
	followerId := 9002
	saverId := 9003
	unrelatedUserId := 9004
	trackId := 9101
	eventId := 9201

	now := time.Now().UTC()
	fixtures := database.FixtureMap{
		"users": []map[string]any{
			{"user_id": ownerId, "handle": "fan_contest_owner"},
			{"user_id": followerId, "handle": "fan_contest_follower"},
			{"user_id": saverId, "handle": "fan_contest_saver"},
			{"user_id": unrelatedUserId, "handle": "fan_contest_unrelated"},
		},
		"tracks": []map[string]any{
			{
				"track_id":    trackId,
				"owner_id":    ownerId,
				"title":       "Unlisted Contest Track",
				"is_unlisted": true,
				"created_at":  now,
				"updated_at":  now,
			},
		},
		"follows": []map[string]any{
			{
				"follower_user_id": followerId,
				"followee_user_id": ownerId,
				"created_at":       now.Add(-time.Hour),
			},
		},
		"saves": []map[string]any{
			{
				"user_id":      saverId,
				"save_item_id": trackId,
				"save_type":    "track",
				"created_at":   now.Add(-time.Hour),
			},
		},
		"events": []map[string]any{
			{
				"event_id":   eventId,
				"event_type": "remix_contest",
				"entity_id":  trackId,
				"user_id":    ownerId,
				"created_at": now,
				"end_date":   now.Add(7 * 24 * time.Hour),
			},
		},
	}

	database.Seed(app.pool.Replicas[0], fixtures)

	// Sanity check: the on_event trigger should NOT have created fan notifications
	// while the track was still unlisted.
	var preFlipCount int
	err := app.writePool.QueryRow(ctx, `
		SELECT count(*) FROM notification
		WHERE type = 'fan_remix_contest_started'
		  AND group_id = 'fan_remix_contest_started:' || $1::int || ':user:' || $2::int
	`, trackId, ownerId).Scan(&preFlipCount)
	require.NoError(t, err)
	assert.Equal(t, 0, preFlipCount, "no fan_remix_contest_started notifications should exist while track is unlisted")

	// Flip the track to public. This should fire handle_track's new unlisted->public
	// branch and create fan_remix_contest_started notifications.
	_, err = app.writePool.Exec(ctx,
		`UPDATE tracks SET is_unlisted = false WHERE track_id = $1 AND is_current = true`,
		trackId,
	)
	require.NoError(t, err)

	// Both the follower and the saver should have received a notification.
	type notifRow struct {
		Specifier string
		GroupId   string
		UserIds   []int32
		EntityId  int
		OwnerId   int
	}
	rows, err := app.writePool.Query(ctx, `
		SELECT specifier, group_id, user_ids,
		       (data->>'entity_id')::int,
		       (data->>'entity_user_id')::int
		FROM notification
		WHERE type = 'fan_remix_contest_started'
		  AND group_id = 'fan_remix_contest_started:' || $1::int || ':user:' || $2::int
		ORDER BY specifier ASC
	`, trackId, ownerId)
	require.NoError(t, err)
	defer rows.Close()

	var got []notifRow
	for rows.Next() {
		var r notifRow
		require.NoError(t, rows.Scan(&r.Specifier, &r.GroupId, &r.UserIds, &r.EntityId, &r.OwnerId))
		got = append(got, r)
	}
	require.NoError(t, rows.Err())

	require.Len(t, got, 2, "expected exactly one notification per notified user (follower, saver)")

	notifiedUserIds := map[int32]bool{}
	for _, r := range got {
		assert.Equal(t,
			"fan_remix_contest_started:9101:user:9001",
			r.GroupId,
			"group_id must match handle_event's format",
		)
		assert.Equal(t, trackId, r.EntityId)
		assert.Equal(t, ownerId, r.OwnerId)
		require.Len(t, r.UserIds, 1)
		notifiedUserIds[r.UserIds[0]] = true
	}
	assert.True(t, notifiedUserIds[int32(followerId)], "follower should have been notified")
	assert.True(t, notifiedUserIds[int32(saverId)], "saver should have been notified")
	assert.False(t, notifiedUserIds[int32(unrelatedUserId)], "unrelated user should not have been notified")
}

func TestTrackRemixesInvalidParams(t *testing.T) {
	app := emptyTestApp(t)

	status, _ := testGet(t, app, "/v1/full/tracks/123/remixes?sort_method=invalid")
	assert.Equal(t, 400, status)

	status, _ = testGet(t, app, "/v1/full/tracks/123/remixes?limit=-1")
	assert.Equal(t, 400, status)

	status, _ = testGet(t, app, "/v1/full/tracks/123/remixes?limit=101")
	assert.Equal(t, 400, status)

	status, _ = testGet(t, app, "/v1/full/tracks/123/remixes?offset=-1")
	assert.Equal(t, 400, status)

	status, _ = testGet(t, app, "/v1/full/tracks/123/remixes?only_contest_entries=invalid")
	assert.Equal(t, 400, status)

	status, _ = testGet(t, app, "/v1/full/tracks/123/remixes?only_cosigns=invalid")
	assert.Equal(t, 400, status)
}
