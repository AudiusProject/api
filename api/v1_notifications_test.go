package api

import (
	"context"
	"strconv"
	"testing"
	"time"

	"api.audius.co/database"
	"api.audius.co/trashid"
	"github.com/stretchr/testify/assert"
)

func TestV1Notifications(t *testing.T) {
	app := emptyTestApp(t)

	fixtures := database.FixtureMap{
		"notification": []map[string]any{
			{
				"id":        1,
				"specifier": "111",
				"group_id":  "tip_send:user_id:111:signature:eee",
				"type":      "tip_send",
				"user_ids":  []int{1},
				"data":      []byte(`{"amount": 100000000, "tx_signature": "asdf", "sender_user_id": 111, "receiver_user_id": 222}`),
			},
			{
				"id":        2,
				"specifier": "128608",
				"group_id":  "milestone:PLAYLIST_REPOST_COUNT:id:128608:threshold:10",
				"type":      "milestone",
				"user_ids":  []int{1},
				"data":      []byte(`{"type": "PLAYLIST_REPOST_COUNT", "threshold": 10, "playlist_id": 128608} `),
			},
			{
				"id":        3,
				"specifier": "100",
				"group_id":  "usdc_purchase_seller:seller_user_id:1:buyer_user_id:100:content_id:1118440:content_type:track",
				"type":      "usdc_purchase_seller",
				"user_ids":  []int{1},
				"data":      []byte(`{"amount": 3000000, "vendor": "user_bank", "content_id": 1118440, "content_type": "track", "extra_amount": 0, "buyer_user_id": 100, "seller_user_id": 1}`),
			},
		},
	}

	database.Seed(app.pool.Replicas[0], fixtures)

	status, body := testGet(t, app, "/v1/notifications/"+trashid.MustEncodeHashID(1))
	assert.Equal(t, 200, status)

	jsonAssert(t, body, map[string]any{
		"data.notifications.0.type":                          "usdc_purchase_seller",
		"data.notifications.0.actions.0.specifier":           trashid.MustEncodeHashID(100),
		"data.notifications.0.actions.0.data.amount":         "3000000",
		"data.notifications.0.actions.0.data.vendor":         "user_bank",
		"data.notifications.0.actions.0.data.content_id":     trashid.MustEncodeHashID(1118440),
		"data.notifications.0.actions.0.data.content_type":   "track",
		"data.notifications.0.actions.0.data.extra_amount":   "0",
		"data.notifications.0.actions.0.data.buyer_user_id":  trashid.MustEncodeHashID(100),
		"data.notifications.0.actions.0.data.seller_user_id": trashid.MustEncodeHashID(1),

		"data.notifications.1.type":                            "tip_send",
		"data.notifications.1.actions.0.specifier":             trashid.MustEncodeHashID(111),
		"data.notifications.1.actions.0.data.amount":           "1000000000000000000",
		"data.notifications.1.actions.0.data.tip_tx_signature": "asdf",
		"data.notifications.1.actions.0.data.sender_user_id":   trashid.MustEncodeHashID(111),
		"data.notifications.1.actions.0.data.receiver_user_id": trashid.MustEncodeHashID(222),

		"data.notifications.2.type":                       "milestone",
		"data.notifications.2.actions.0.specifier":        trashid.MustEncodeHashID(128608),
		"data.notifications.2.actions.0.data.type":        "playlist_repost_count",
		"data.notifications.2.actions.0.data.is_album":    false,
		"data.notifications.2.actions.0.data.playlist_id": trashid.MustEncodeHashID(128608),
	})
}

func TestV1Notifications_NotDeletedTrack(t *testing.T) {
	app := emptyTestApp(t)

	fixtures := database.FixtureMap{
		"tracks": []map[string]any{
			{
				"track_id":  67576,
				"owner_id":  10,
				"is_delete": false,
			},
		},
		"notification": []map[string]any{
			{
				"id":        1,
				"specifier": "67576",
				"group_id":  "create:track:user_id:67576",
				"type":      "create",
				"user_ids":  []int{1},
				"data":      []byte(`{"track_id": 67576}`),
			},
			{
				"id":        2,
				"specifier": "190321",
				"group_id":  "milestone:PLAYLIST_REPOST_COUNT:id:128608:threshold:10",
				"type":      "milestone",
				"user_ids":  []int{1},
				"data":      []byte(`{"type": "PLAYLIST_REPOST_COUNT", "threshold": 10, "playlist_id": 128608} `),
			},
		},
	}

	database.Seed(app.pool.Replicas[0], fixtures)

	status, body := testGet(t, app, "/v1/notifications/"+trashid.MustEncodeHashID(1))
	assert.Equal(t, 200, status)

	jsonAssert(t, body, map[string]any{
		"data.notifications.#":      2,
		"data.notifications.0.type": "milestone",
		"data.notifications.1.type": "create",
	})
}

func TestV1Notifications_DeletedTrack(t *testing.T) {
	app := emptyTestApp(t)

	fixtures := database.FixtureMap{
		"tracks": []map[string]any{
			{
				"track_id":  67576,
				"owner_id":  10,
				"is_delete": true,
			},
		},
		"notification": []map[string]any{
			{
				"id":        1,
				"specifier": "67576",
				"group_id":  "create:track:user_id:67576",
				"type":      "create",
				"user_ids":  []int{1},
				"data":      []byte(`{"track_id": 67576}`),
			},
			{
				"id":        2,
				"specifier": "190321",
				"group_id":  "milestone:PLAYLIST_REPOST_COUNT:id:128608:threshold:10",
				"type":      "milestone",
				"user_ids":  []int{1},
				"data":      []byte(`{"type": "PLAYLIST_REPOST_COUNT", "threshold": 10, "playlist_id": 128608} `),
			},
		},
	}

	database.Seed(app.pool.Replicas[0], fixtures)

	status, body := testGet(t, app, "/v1/notifications/"+trashid.MustEncodeHashID(1))
	assert.Equal(t, 200, status)

	jsonAssert(t, body, map[string]any{
		"data.notifications.#":      1,
		"data.notifications.0.type": "milestone",
	})
}

func TestV1Notifications_DeletedPlaylist(t *testing.T) {
	app := emptyTestApp(t)

	fixtures := database.FixtureMap{
		"playlists": []map[string]any{
			{
				"playlist_id":       67576,
				"playlist_owner_id": 10,
				"is_delete":         true,
			},
		},
		"notification": []map[string]any{
			{
				"id":        1,
				"specifier": "67576",
				"group_id":  "create:playlist:user_id:67576",
				"type":      "create",
				"user_ids":  []int{1},
				"data":      []byte(`{"playlist_id": 67576}`),
			},
			{
				"id":        2,
				"specifier": "190321",
				"group_id":  "milestone:PLAYLIST_REPOST_COUNT:id:128608:threshold:10",
				"type":      "milestone",
				"user_ids":  []int{1},
				"data":      []byte(`{"type": "PLAYLIST_REPOST_COUNT", "threshold": 10, "playlist_id": 128608} `),
			},
		},
	}

	database.Seed(app.pool.Replicas[0], fixtures)

	status, body := testGet(t, app, "/v1/notifications/"+trashid.MustEncodeHashID(1))
	assert.Equal(t, 200, status)

	jsonAssert(t, body, map[string]any{
		"data.notifications.#":      1,
		"data.notifications.0.type": "milestone",
	})
}

func TestV1Notifications_Comment(t *testing.T) {
	app := emptyTestApp(t)

	fixtures := database.FixtureMap{
		"notification": []map[string]any{
			{
				"id":        1,
				"specifier": "67576",
				"group_id":  "comment:track:user_id:67576:comment_id:1",
				"type":      "comment",
				"user_ids":  []int{1},
				"data":      []byte(`{"comment_id": 1, "type": "Track"}`),
			},
		},
	}

	database.Seed(app.pool.Replicas[0], fixtures)

	status, body := testGet(t, app, "/v1/notifications/"+trashid.MustEncodeHashID(1))
	assert.Equal(t, 200, status)

	jsonAssert(t, body, map[string]any{
		"data.notifications.#":                     1,
		"data.notifications.0.type":                "comment",
		"data.notifications.0.actions.0.data.type": "Track",
	})
}

func TestV1Notifications_DeactivatedUser(t *testing.T) {
	app := emptyTestApp(t)

	fixtures := database.FixtureMap{
		"users": []map[string]any{
			{
				"user_id":        67576,
				"is_deactivated": true,
			},
			{
				"user_id":        1235,
				"is_deactivated": false,
			},
		},
		"notification": []map[string]any{
			{ // Deactivated user
				"id":        1,
				"specifier": "1234",
				"group_id":  "comment:track:user_id:67576:comment_id:1",
				"type":      "comment",
				"user_ids":  []int{1},
				"data":      []byte(`{"comment_id": 1, "type": "Track", "entity_user_id": 67576}`),
			},
			{
				"id":        2,
				"specifier": "1235",
				"group_id":  "comment:track:user_id:1235:comment_id:1",
				"type":      "comment",
				"user_ids":  []int{1},
				"data":      []byte(`{"comment_id": 1, "type": "Track", "entity_user_id": 1235}`),
			},
		},
	}

	database.Seed(app.pool.Replicas[0], fixtures)

	status, body := testGet(t, app, "/v1/notifications/"+trashid.MustEncodeHashID(1))
	assert.Equal(t, 200, status)

	jsonAssert(t, body, map[string]any{
		"data.notifications.#":                     1,
		"data.notifications.0.type":                "comment",
		"data.notifications.0.actions.0.specifier": trashid.MustEncodeHashID(1235),
	})
}

func TestV1Notifications_LowScore(t *testing.T) {
	app := emptyTestApp(t)

	fixtures := database.FixtureMap{
		"users": []map[string]any{
			{
				"user_id":        67576,
				"is_deactivated": false,
			},
			{
				"user_id":        1235,
				"is_deactivated": false,
			},
		},
		"aggregate_user": []map[string]any{
			{
				"user_id": 67576,
				"score":   -1,
			},
			{
				"user_id": 1235,
				"score":   0,
			},
		},
		"notification": []map[string]any{
			{ // Low score
				"id":        1,
				"specifier": "67576",
				"group_id":  "comment:track:user_id:67576:comment_id:1",
				"type":      "comment",
				"user_ids":  []int{1},
				"data":      []byte(`{"comment_id": 1, "type": "Track", "entity_user_id": 67576}`),
			},
			{
				"id":        2,
				"specifier": "1235",
				"group_id":  "comment:track:user_id:1235:comment_id:1",
				"type":      "comment",
				"user_ids":  []int{1},
				"data":      []byte(`{"comment_id": 1, "type": "Track", "entity_user_id": 1235}`),
			},
		},
	}

	database.Seed(app.pool.Replicas[0], fixtures)

	status, body := testGet(t, app, "/v1/notifications/"+trashid.MustEncodeHashID(1))
	assert.Equal(t, 200, status)

	jsonAssert(t, body, map[string]any{
		"data.notifications.#":                     1,
		"data.notifications.0.type":                "comment",
		"data.notifications.0.actions.0.specifier": trashid.MustEncodeHashID(1235),
	})
}

func TestV1Notifications_LowScoreManagerRequest(t *testing.T) {
	app := emptyTestApp(t)

	const recipientUserID = 1
	const lowScoreGrantorUserID = 67576

	fixtures := database.FixtureMap{
		"users": []map[string]any{
			{
				"user_id":        lowScoreGrantorUserID,
				"is_deactivated": false,
			},
		},
		"aggregate_user": []map[string]any{
			{
				"user_id": lowScoreGrantorUserID,
				"score":   -1,
			},
		},
		"notification": []map[string]any{
			{
				"id":        1,
				"specifier": strconv.Itoa(lowScoreGrantorUserID),
				"group_id":  "request_manager:grantee_user_id:1:user_id:67576",
				"type":      "request_manager",
				"user_ids":  []int{recipientUserID},
				"data": []byte(
					`{"user_id": 67576, "grantee_address": "0xabc", "grantee_user_id": 1}`,
				),
			},
		},
	}

	database.Seed(app.pool.Replicas[0], fixtures)

	status, body := testGet(
		t,
		app,
		"/v1/notifications/"+
			trashid.MustEncodeHashID(recipientUserID)+
			"?types=request_manager",
	)
	assert.Equal(t, 200, status)

	jsonAssert(t, body, map[string]any{
		"data.notifications.#":                                1,
		"data.notifications.0.type":                           "request_manager",
		"data.notifications.0.actions.0.specifier":            trashid.MustEncodeHashID(lowScoreGrantorUserID),
		"data.notifications.0.actions.0.data.user_id":         trashid.MustEncodeHashID(lowScoreGrantorUserID),
		"data.notifications.0.actions.0.data.grantee_user_id": trashid.MustEncodeHashID(recipientUserID),
	})
}

func TestV1Notifications_UnlistedTrack(t *testing.T) {
	app := emptyTestApp(t)

	fixtures := database.FixtureMap{
		"users": []map[string]any{
			{
				"user_id":        67576,
				"is_deactivated": true,
			},
		},
		"tracks": []map[string]any{
			{
				"track_id":    1,
				"owner_id":    10,
				"is_unlisted": false,
			},
			{
				"track_id":    2,
				"owner_id":    10,
				"is_unlisted": true,
			},
		},
		"notification": []map[string]any{
			{
				"id":        1,
				"specifier": trashid.MustEncodeHashID(1),
				"group_id":  "create:track:user_id:10",
				"type":      "create",
				"user_ids":  []int{1},
				"data":      []byte(`{"track_id": 1}`),
			},
			{
				"id":        2,
				"specifier": trashid.MustEncodeHashID(2),
				"group_id":  "create:track:user_id:10",
				"type":      "create",
				"user_ids":  []int{1},
				"data":      []byte(`{"track_id": 2}`),
			},
		},
	}

	database.Seed(app.pool.Replicas[0], fixtures)

	status, body := testGet(t, app, "/v1/notifications/"+trashid.MustEncodeHashID(1))
	assert.Equal(t, 200, status)

	jsonAssert(t, body, map[string]any{
		"data.notifications.#":                     1,
		"data.notifications.0.type":                "create",
		"data.notifications.0.actions.0.specifier": trashid.MustEncodeHashID(1),
	})
}

func TestV1Notifications_PrivatePlaylist(t *testing.T) {
	app := emptyTestApp(t)

	fixtures := database.FixtureMap{
		"playlists": []map[string]any{
			{
				"playlist_id":       67576,
				"playlist_owner_id": 10,
				"is_private":        true,
			},
		},
		"notification": []map[string]any{
			{
				"id":        1,
				"specifier": trashid.MustEncodeHashID(67576),
				"group_id":  "create:playlist:user_id:67576",
				"type":      "create",
				"user_ids":  []int{1},
				"data":      []byte(`{"playlist_id": 67576}`),
			},
			{
				"id":        2,
				"specifier": trashid.MustEncodeHashID(67576),
				"group_id":  "milestone:PLAYLIST_REPOST_COUNT:id:128608:threshold:10",
				"type":      "milestone",
				"user_ids":  []int{1},
				"data":      []byte(`{"type": "PLAYLIST_REPOST_COUNT", "threshold": 10, "playlist_id": 128608} `),
			},
		},
	}

	database.Seed(app.pool.Replicas[0], fixtures)

	status, body := testGet(t, app, "/v1/notifications/"+trashid.MustEncodeHashID(1))
	assert.Equal(t, 200, status)

	jsonAssert(t, body, map[string]any{
		"data.notifications.#":      1,
		"data.notifications.0.type": "milestone",
	})
}

func TestV1Notifications_AnnouncementRequiresUserIdInUserIds(t *testing.T) {
	app := emptyTestApp(t)

	fixtures := database.FixtureMap{
		"notification": []map[string]any{
			{
				"id":        1,
				"specifier": "1",
				"group_id":  "announcement:target-user-1",
				"type":      "announcement",
				"user_ids":  []int{1},
				"data":      []byte(`{"title": "For user 1", "short_description": "hi"}`),
			},
			{
				"id":        2,
				"specifier": "2",
				"group_id":  "announcement:target-user-2",
				"type":      "announcement",
				"user_ids":  []int{2},
				"data":      []byte(`{"title": "For user 2", "short_description": "bye"}`),
			},
			{
				"id":        3,
				"specifier": "3",
				"group_id":  "announcement:empty-user-ids",
				"type":      "announcement",
				"user_ids":  []int{},
				"data":      []byte(`{"title": "Nobody", "short_description": "x"}`),
			},
		},
	}

	database.Seed(app.pool.Replicas[0], fixtures)

	status, body := testGet(t, app, "/v1/notifications/"+trashid.MustEncodeHashID(1))
	assert.Equal(t, 200, status)

	jsonAssert(t, body, map[string]any{
		"data.notifications.#":                      1,
		"data.notifications.0.type":                 "announcement",
		"data.notifications.0.group_id":             "announcement:target-user-1",
		"data.notifications.0.actions.0.data.title": "For user 1",
	})
}

func TestV1Notifications_TrackCollaboratorInviteIncludesCurrentStatus(t *testing.T) {
	app := emptyTestApp(t)
	ctx := context.Background()

	const trackID = 700
	const collaboratorUserID = 1
	const inviterUserID = 500

	invitedAt := time.Now()
	_, err := app.pool.Replicas[0].Exec(ctx, `
		INSERT INTO track_collaborators (
			track_id,
			collaborator_user_id,
			invited_by,
			status,
			created_at,
			updated_at,
			txhash
		)
		VALUES ($1, $2, $3, 'pending', $4, $4, 'invite-tx')
	`, trackID, collaboratorUserID, inviterUserID, invitedAt)
	assert.NoError(t, err)

	_, err = app.pool.Replicas[0].Exec(ctx, `
		UPDATE track_collaborators
		SET status = 'accepted', updated_at = $3
		WHERE track_id = $1 AND collaborator_user_id = $2
	`, trackID, collaboratorUserID, invitedAt.Add(time.Second))
	assert.NoError(t, err)

	status, body := testGet(
		t,
		app,
		"/v1/notifications/"+
			trashid.MustEncodeHashID(collaboratorUserID)+
			"?types=track_collaborator_invite",
	)
	assert.Equal(t, 200, status)

	jsonAssert(t, body, map[string]any{
		"data.notifications.#":                                     1,
		"data.notifications.0.type":                                "track_collaborator_invite",
		"data.notifications.0.actions.0.data.track_id":             trashid.MustEncodeHashID(trackID),
		"data.notifications.0.actions.0.data.collaborator_user_id": trashid.MustEncodeHashID(collaboratorUserID),
		"data.notifications.0.actions.0.data.inviter_user_id":      trashid.MustEncodeHashID(inviterUserID),
		"data.notifications.0.actions.0.data.status":               "accepted",
	})
}

// TestV1Notifications_RelatedEntities exercises the response's `related` block:
//
//   - users/tracks/playlists referenced by notification action data are
//     hydrated server-side so the client doesn't need follow-up round trips
//   - actor IDs are capped at notificationRelatedActorsPerGroup per group so
//     a fan-out notification (e.g. 100 followers) doesn't bloat the response;
//     the target entity (the followee, in this case) is duplicated in every
//     action's data so it's still picked up under the cap
//   - polymorphic *_item_id fields (repost_item_id here) are routed to the
//     right bucket based on the sibling `type` discriminator
func TestV1Notifications_RelatedEntities(t *testing.T) {
	app := emptyTestApp(t)

	const recipient = 1
	// Five followers, but the per-group cap should drop us to
	// notificationRelatedActorsPerGroup followers + the followee.
	followers := []int{100, 101, 102, 103, 104}
	const reposter = 300
	const repostedTrackID = 50
	const repostedTrackOwner = 200
	const savedPlaylistID = 60
	const saver = 400

	users := []map[string]any{
		{"user_id": recipient},
		{"user_id": reposter},
		{"user_id": repostedTrackOwner},
		{"user_id": saver},
	}
	for _, fid := range followers {
		users = append(users, map[string]any{"user_id": fid})
	}

	// timestamp is intentionally omitted — the seed default (time.Now()) keeps
	// these notifications inside the SQL handler's 90-day initial-load window.
	notifs := []map[string]any{
		{
			"id":        10,
			"specifier": "300",
			"group_id":  "repost:track:50",
			"type":      "repost",
			"user_ids":  []int{recipient},
			"data":      []byte(`{"type": "track", "user_id": 300, "repost_item_id": 50}`),
		},
		{
			"id":        11,
			"specifier": "400",
			"group_id":  "save:playlist:60",
			"type":      "save",
			"user_ids":  []int{recipient},
			"data":      []byte(`{"type": "playlist", "user_id": 400, "save_item_id": 60}`),
		},
	}
	// Five follow notifications, all in the same group (one logical
	// "you got followed by 5 people" notification after json_agg).
	for i, fid := range followers {
		notifs = append(notifs, map[string]any{
			"id":        20 + i,
			"specifier": strconv.Itoa(fid),
			"group_id":  "follow:1",
			"type":      "follow",
			"user_ids":  []int{recipient},
			"data": []byte(`{"follower_user_id": ` + strconv.Itoa(fid) +
				`, "followee_user_id": ` + strconv.Itoa(recipient) + `}`),
		})
	}

	fixtures := database.FixtureMap{
		"users":  users,
		"tracks": []map[string]any{{"track_id": repostedTrackID, "owner_id": repostedTrackOwner}},
		"playlists": []map[string]any{
			{"playlist_id": savedPlaylistID, "playlist_owner_id": recipient},
		},
		"notification": notifs,
	}

	database.Seed(app.pool.Replicas[0], fixtures)

	status, body := testGet(t, app, "/v1/notifications/"+trashid.MustEncodeHashID(recipient))
	assert.Equal(t, 200, status)

	gotTrackIds := pluckStrings(body, "related.tracks.#.id")
	assert.ElementsMatch(t,
		[]string{trashid.MustEncodeHashID(repostedTrackID)},
		gotTrackIds,
		"reposted track must be hydrated under related.tracks",
	)

	gotPlaylistIds := pluckStrings(body, "related.playlists.#.id")
	assert.ElementsMatch(t,
		[]string{trashid.MustEncodeHashID(savedPlaylistID)},
		gotPlaylistIds,
		"saved playlist must be hydrated under related.playlists",
	)

	gotUserIds := pluckStrings(body, "related.users.#.id")

	// Fan-out cap: at most notificationRelatedActorsPerGroup followers from the
	// follow group, plus the reposter, the saver, and the followee (recipient).
	maxFollowersHydrated := notificationRelatedActorsPerGroup
	maxExpected := maxFollowersHydrated + 3 // reposter, saver, followee
	assert.LessOrEqual(t, len(gotUserIds), maxExpected,
		"actor cap must bound the related.users size for fan-out groups; got %v", gotUserIds)

	// Always-included targets: the recipient (followee), the reposter, the saver.
	assert.Contains(t, gotUserIds, trashid.MustEncodeHashID(recipient),
		"followee (recipient) must appear in related.users")
	assert.Contains(t, gotUserIds, trashid.MustEncodeHashID(reposter),
		"reposter must appear in related.users")
	assert.Contains(t, gotUserIds, trashid.MustEncodeHashID(saver),
		"saver must appear in related.users")
}
