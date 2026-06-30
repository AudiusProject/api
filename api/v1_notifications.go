package api

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"time"

	"api.audius.co/api/dbv1"
	"api.audius.co/trashid"
	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

// Per-group cap on how many actions we mine for actor user IDs. Notification
// groups can fan out (e.g. one row representing 100 followers); the client
// only renders one avatar per group, so a single actor profile is enough.
// Target entity IDs (the followee, the reposted track, etc.) are duplicated
// across every action in a group, so reading just the first action still
// surfaces every target — only the actor list is bounded by this cap.
const notificationRelatedActorsPerGroup = 1

const (
	// Keep notification reads well below the production replica's
	// max_standby_streaming_delay so pathological reads fail fast instead of
	// holding hot-standby snapshots and making the replica fall behind.
	notificationReadTimeout = 8 * time.Second

	// Historically `limit=0` fell back to the default limit of 20, so unread
	// polling counted at most the first page of notification groups.
	notificationUnreadPollLimit = 20
)

type GetNotificationsQueryParams struct {
	Limit     int      `query:"limit" default:"20" validate:"min=0,max=100"`
	Types     []string `query:"types" validate:"dive,oneof=announcement follow repost save remix cosign create tip_receive tip_send challenge_reward repost_of_repost save_of_repost tastemaker reaction supporter_dethroned supporter_rank_up supporting_rank_up milestone track_added_to_playlist tier_change trending trending_playlist trending_underground usdc_purchase_buyer usdc_purchase_seller track_added_to_purchased_album request_manager approve_manager_request track_collaborator_invite track_collaborator_accept claimable_reward comment comment_thread comment_mention comment_reaction listen_streak_reminder fan_remix_contest_started fan_remix_contest_ended fan_remix_contest_ending_soon fan_remix_contest_winners_selected fan_remix_contest_submission artist_remix_contest_ended artist_remix_contest_ending_soon artist_remix_contest_submissions fan_club_text_post remix_contest_update"`
	GroupID   string   `query:"group_id" validate:"omitempty"`
	Timestamp float64  `query:"timestamp" validate:"omitempty,min=0"`
}

type notificationRow struct {
	Type    string            `json:"type"`
	GroupID string            `json:"group_id"`
	Actions []json.RawMessage `json:"actions"`
	IsSeen  bool              `json:"is_seen"`
	SeenAt  interface{}       `json:"seen_at"`
}

var unsupportedNotificationTypes = []string{
	// No frontend support
	"usdc_transfer",
	"usdc_withdrawal",
	"reward_in_cooldown",
	// Deprecated
	"milestone_follower_count",
	"remix_contest_started",
	"remix_contest_ending_soon",
	"claimble_reward",
	// Broken
	"remix",
}

func (app *ApiServer) v1NotificationsUnreadPoll(c *fiber.Ctx, params GetNotificationsQueryParams) error {
	sql := `
WITH latest_seen AS (
	SELECT MAX(seen_at) AS seen_at
	FROM notification_seen
	WHERE user_id = @user_id
),
-- Equivalent to ARRAY[@user_id] && n.user_ids, split so single-recipient rows
-- can use notification_single_recipient_user_timestamp_idx while
-- multi-recipient arrays keep the existing overlap semantics.
matched_notifications AS (
	SELECT n.type, n.group_id
	FROM notification n
	CROSS JOIN latest_seen
	WHERE array_length(n.user_ids, 1) = 1
		AND n.user_ids[1] = @user_id
		AND n.timestamp > (now()::timestamp - interval '90 days')
		AND n.timestamp > COALESCE(latest_seen.seen_at, '-infinity'::timestamp)
		AND (n.type = ANY(@types) OR @types IS NULL)
		AND (n.type != ALL(@unsupported_types))
	UNION ALL
	SELECT n.type, n.group_id
	FROM notification n
	CROSS JOIN latest_seen
	WHERE COALESCE(array_length(n.user_ids, 1), 0) != 1
		AND ARRAY[@user_id] && n.user_ids
		AND n.timestamp > (now()::timestamp - interval '90 days')
		AND n.timestamp > COALESCE(latest_seen.seen_at, '-infinity'::timestamp)
		AND (n.type = ANY(@types) OR @types IS NULL)
		AND (n.type != ALL(@unsupported_types))
)
SELECT COUNT(*)
FROM (
	SELECT 1
	FROM matched_notifications
	GROUP BY type, group_id
	LIMIT @limit
) unread_notifications;
`
	ctx, cancel := context.WithTimeout(c.Context(), notificationReadTimeout)
	defer cancel()

	var unreadCount int
	err := app.pool.QueryRow(ctx, sql, pgx.NamedArgs{
		"user_id":           app.getUserId(c),
		"limit":             notificationUnreadPollLimit,
		"types":             params.Types,
		"unsupported_types": unsupportedNotificationTypes,
	}).Scan(&unreadCount)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return fiber.NewError(fiber.StatusGatewayTimeout, "notifications unread query timed out")
		}
		return err
	}

	return c.JSON(fiber.Map{
		"data": fiber.Map{
			"notifications": []notificationRow{},
			"unread_count":  unreadCount,
		},
		"related": fiber.Map{
			"users":     []any{},
			"tracks":    []any{},
			"playlists": []any{},
		},
	})
}

func (app *ApiServer) v1Notifications(c *fiber.Ctx) error {
	limitZeroPoll := c.Query("limit") == "0"

	params := GetNotificationsQueryParams{}
	if err := app.ParseAndValidateQueryParams(c, &params); err != nil {
		return err
	}

	if limitZeroPoll {
		return app.v1NotificationsUnreadPoll(c, params)
	}

	sql := `
-- user_seen is a window function that gets windows between seen events.
--
-- seen_at	              prev_seen_at
-- now()	                "2025-08-05 16:27:53"
-- "2025-08-05 16:27:53"	"2025-08-04 21:50:38"
-- "2025-08-04 21:50:38"	"2025-08-04 18:12:41"
--
WITH user_seen as (
  SELECT
    LAG(seen_at, 1, now()::timestamp) OVER ( ORDER BY seen_at desc ) AS seen_at,
    seen_at as prev_seen_at
  FROM
    notification_seen
  WHERE
    user_id = @user_id
  ORDER BY
    seen_at desc
  LIMIT 10
),
-- Equivalent to ARRAY[@user_id] && n.user_ids, split so single-recipient rows
-- can use notification_single_recipient_user_timestamp_idx while
-- multi-recipient arrays keep the existing overlap semantics.
matched_notifications AS (
	SELECT
		n.id,
		n.specifier,
		n.group_id,
		n.type,
		n.timestamp,
		n.data
	FROM notification n
	WHERE
		array_length(n.user_ids, 1) = 1
		AND n.user_ids[1] = @user_id
		AND (n.type = ANY(@types) OR @types IS NULL)
		AND (n.type != ALL(@unsupported_types))
		AND (
			-- Initial load: bound to the last 90 days so heavy users don't fan out
			-- over their entire notification history. Pagination (timestamp_offset > 0)
			-- is unbounded so scrolling further back still works.
			(@timestamp_offset = 0 AND @group_id_offset = '' AND n.timestamp > (now()::timestamp - interval '90 days')) OR
			(@timestamp_offset = 0 AND @group_id_offset != '' AND n.group_id < @group_id_offset) OR
			(@timestamp_offset > 0 AND n.timestamp < to_timestamp(@timestamp_offset)) OR
			(
				@group_id_offset != '' AND @timestamp_offset > 0 AND
				(n.timestamp = to_timestamp(@timestamp_offset) AND n.group_id < @group_id_offset)
			)
		)
	UNION ALL
	SELECT
		n.id,
		n.specifier,
		n.group_id,
		n.type,
		n.timestamp,
		n.data
	FROM notification n
	WHERE
		COALESCE(array_length(n.user_ids, 1), 0) != 1
		AND ARRAY[@user_id] && n.user_ids
		AND (n.type = ANY(@types) OR @types IS NULL)
		AND (n.type != ALL(@unsupported_types))
		AND (
			-- Initial load: bound to the last 90 days so heavy users don't fan out
			-- over their entire notification history. Pagination (timestamp_offset > 0)
			-- is unbounded so scrolling further back still works.
			(@timestamp_offset = 0 AND @group_id_offset = '' AND n.timestamp > (now()::timestamp - interval '90 days')) OR
			(@timestamp_offset = 0 AND @group_id_offset != '' AND n.group_id < @group_id_offset) OR
			(@timestamp_offset > 0 AND n.timestamp < to_timestamp(@timestamp_offset)) OR
			(
				@group_id_offset != '' AND @timestamp_offset > 0 AND
				(n.timestamp = to_timestamp(@timestamp_offset) AND n.group_id < @group_id_offset)
			)
		)
)
SELECT
	n.type,
	n.group_id AS group_id,
	json_agg(
		json_build_object(
			'type', n.type,
			'specifier', n.specifier,
			'timestamp', EXTRACT(EPOCH FROM n.timestamp),
			'data',
				CASE
					WHEN n.type = 'track_collaborator_invite' AND tc.status IS NOT NULL
					THEN jsonb_set(n.data, '{status}', to_jsonb(tc.status), true)
					ELSE n.data
				END
		)
		ORDER BY n.timestamp DESC
	)::jsonb AS actions,
	CASE
		-- If seen at is not null, we were able to match a window between seen events
		WHEN user_seen.seen_at IS NOT NULL THEN
			CASE
			  -- In all cases except the most recent window, this means we've already seen
				-- the notification
				WHEN now()::timestamp != user_seen.seen_at THEN true
				ELSE false
			END
		-- Otherwise, we've only seen notifications before if we have some row in notification_seen
		ELSE EXISTS(SELECT 1 from notification_seen ns WHERE ns.user_id = @user_id)
	END::boolean AS is_seen,
	CASE
		WHEN user_seen.seen_at != now()::timestamp THEN EXTRACT(EPOCH FROM user_seen.seen_at)
		ELSE null
	END AS seen_at
FROM
    matched_notifications n
LEFT JOIN user_seen ON
  user_seen.seen_at >= n.timestamp AND user_seen.prev_seen_at < n.timestamp
-- Join with tracks table to filter out deleted tracks for "create" notifications that have track_id
LEFT JOIN tracks t ON
  n.type = 'create' AND
  n.data ? 'track_id' AND
  t.track_id = (n.data->>'track_id')::integer AND
  t.is_current = true
-- Join with playlists table to filter out deleted playlists for "create" notifications that have playlist_id
LEFT JOIN playlists p ON
  n.type = 'create' AND
  n.data ? 'playlist_id' AND
  p.playlist_id = (n.data->>'playlist_id')::integer AND
  p.is_current = true
LEFT JOIN track_collaborators tc ON
  n.type = 'track_collaborator_invite' AND
  n.data ? 'track_id' AND
  n.data ? 'collaborator_user_id' AND
  tc.track_id = (n.data->>'track_id')::integer AND
  tc.collaborator_user_id = (n.data->>'collaborator_user_id')::integer
WHERE
  -- Filter out notifications for deleted tracks (only for create notifications that have track_id)
  (
		n.type != 'create'
		OR NOT (n.data ? 'track_id')
		OR (t.is_delete = false AND t.is_unlisted = false)
	)
  -- Filter out notifications for deleted playlists (only for create notifications that have playlist_id)
  AND (
		n.type != 'create'
		OR NOT (n.data ? 'playlist_id')
		OR (p.is_delete = false AND p.is_private = false)
	)
	-- Filter out notifications from deleted/low score users
	AND (
		-- If notification has no user data fields, allow it through
		NOT (
			n.data ? 'user_id'
			OR n.data ? 'follower_user_id'
			OR n.data ? 'comment_user_id'
			OR n.data ? 'entity_user_id'
		)
		OR (
			-- If notification has user data fields, ensure ALL referenced users are valid and active with good score
			(
				n.data ? 'user_id'
				OR n.data ? 'follower_user_id'
				OR n.data ? 'comment_user_id'
				OR n.data ? 'entity_user_id'
			)
			AND (
				-- Check user_id if present
				NOT (n.data ? 'user_id') OR EXISTS (
					SELECT 1 FROM users u2
					JOIN aggregate_user a2 ON u2.user_id = a2.user_id
					WHERE u2.user_id = (n.data->>'user_id')::integer
					AND u2.is_current = true
					AND u2.is_deactivated = false
					AND a2.score >= 0
				)
			)
			AND (
				-- Check follower_user_id if present
				NOT (n.data ? 'follower_user_id') OR EXISTS (
					SELECT 1 FROM users u2
					JOIN aggregate_user a2 ON u2.user_id = a2.user_id
					WHERE u2.user_id = (n.data->>'follower_user_id')::integer
					AND u2.is_current = true
					AND u2.is_deactivated = false
					AND a2.score >= 0
				)
			)
			AND (
				-- Check comment_user_id if present
				NOT (n.data ? 'comment_user_id') OR EXISTS (
					SELECT 1 FROM users u2
					JOIN aggregate_user a2 ON u2.user_id = a2.user_id
					WHERE u2.user_id = (n.data->>'comment_user_id')::integer
					AND u2.is_current = true
					AND u2.is_deactivated = false
					AND a2.score >= 0
				)
			)
			AND (
				-- Check entity_user_id if present (skip score check for fan club notifications since the entity user must be verified)
				NOT (n.data ? 'entity_user_id') OR EXISTS (
					SELECT 1 FROM users u2
					JOIN aggregate_user a2 ON u2.user_id = a2.user_id
					WHERE u2.user_id = (n.data->>'entity_user_id')::integer
					AND u2.is_current = true
					AND u2.is_deactivated = false
					AND (n.type = 'fan_club_text_post' OR a2.score >= 0)
				)
			)
		)
	)
GROUP BY
  n.type, n.group_id, user_seen.seen_at, user_seen.prev_seen_at,
  CASE
		-- Group notifications individually that are older than any of the seen windows
		-- and we know that the user has seen at least one notification before
    WHEN user_seen.seen_at IS NULL AND
			EXISTS(SELECT 1 from notification_seen ns WHERE ns.user_id = @user_id)
    THEN n.timestamp
    ELSE NULL
  END
ORDER BY
  user_seen.seen_at desc NULLS LAST,
  max(n.timestamp) desc,
  n.group_id desc
limit @limit::int
;
`
	userId := app.getUserId(c)

	ctx, cancel := context.WithTimeout(c.Context(), notificationReadTimeout)
	defer cancel()

	rows, err := app.pool.Query(ctx, sql, pgx.NamedArgs{
		"user_id":           userId,
		"limit":             params.Limit,
		"types":             params.Types,
		"group_id_offset":   params.GroupID,
		"timestamp_offset":  params.Timestamp,
		"unsupported_types": unsupportedNotificationTypes,
	})
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return fiber.NewError(fiber.StatusGatewayTimeout, "notifications query timed out")
		}
		return err
	}

	notifs, err := pgx.CollectRows(rows, pgx.RowToAddrOfStructByNameLax[notificationRow])
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return fiber.NewError(fiber.StatusGatewayTimeout, "notifications query timed out")
		}
		return err
	}

	userIds := []int32{}
	trackIds := []int32{}
	playlistIds := []int32{}

	unreadCount := 0
	for _, notif := range notifs {

		slices.SortFunc(notif.Actions, func(a, b json.RawMessage) int {
			specA := gjson.GetBytes(a, "specifier").String()
			specB := gjson.GetBytes(b, "specifier").String()
			return strings.Compare(specA, specB)
		})

		// Mine related entity IDs from the first N actions of each group. This
		// must happen BEFORE HashifyJson re-encodes ints as opaque strings.
		mineLimit := len(notif.Actions)
		if mineLimit > notificationRelatedActorsPerGroup {
			mineLimit = notificationRelatedActorsPerGroup
		}
		for _, action := range notif.Actions[:mineLimit] {
			collectNotificationRelatedIds(action, &userIds, &trackIds, &playlistIds)
		}

		// each row from notification table has `actions`
		// which is a jsonb field that is an array of objects.
		// we need to hash encode all id fields (HashifyJson)
		// and do some additional transforms.
		// see extend_notification.py for details
		for idx, action := range notif.Actions {
			action = trashid.HashifyJson(action)

			// lowercase type field if not a comment notification
			if val := gjson.GetBytes(action, "data.type"); val.Exists() &&
				notif.Type != "comment" &&
				notif.Type != "comment_thread" &&
				notif.Type != "comment_mention" &&
				notif.Type != "comment_reaction" {
				action, _ = sjson.SetBytes(action, "data.type", strings.ToLower(val.String()))
			}

			// for playlist milestones: is_album: default to false
			if strings.HasPrefix(notif.GroupID, "milestone:PLAYLIST_") {
				isAlbum := gjson.GetBytes(action, "data.is_album").Bool()
				action, _ = sjson.SetBytes(action, "data.is_album", isAlbum)
			}

			// For notifications in AUDIO, we need to add 0000000000 to the amount field
			// to convert from SPL to wei
			if notif.Type == "tip_send" ||
				notif.Type == "tip_receive" ||
				notif.Type == "challenge_reward" ||
				notif.Type == "claimable_reward" ||
				notif.Type == "reaction" {
				for _, fieldPath := range []string{"data.amount", "data.tip_amount"} {
					if val := gjson.GetBytes(action, fieldPath); val.Exists() {
						action, _ = sjson.SetBytes(action, fieldPath, val.String()+"0000000000")
					}
				}
			}
			// For notifications in $USDC, convert to string, but do not add padding
			if notif.Type == "usdc_purchase_buyer" || notif.Type == "usdc_purchase_seller" {
				for _, fieldPath := range []string{"data.amount", "data.extra_amount"} {
					if val := gjson.GetBytes(action, fieldPath); val.Exists() {
						action, _ = sjson.SetBytes(action, fieldPath, val.String())
					}
				}
			}

			// alias fields to alternate name
			if strings.HasPrefix(notif.Type, "tip_") {
				action, _ = sjson.SetBytes(action, "data.tip_tx_signature", gjson.GetBytes(action, "data.tx_signature").String())
			}

			notif.Actions[idx] = action
		}

		if !notif.IsSeen {
			unreadCount++
		}
	}

	related, err := app.queries.Parallel(c.Context(), dbv1.ParallelParams{
		UserIds:         userIds,
		TrackIds:        trackIds,
		PlaylistIds:     playlistIds,
		MyID:            app.getMyId(c),
		AuthedWallet:    app.tryGetAuthedWallet(c),
		IncludeUnlisted: true,
	})
	if err != nil {
		return err
	}

	return c.JSON(fiber.Map{
		"data": fiber.Map{
			"notifications": notifs,
			"unread_count":  unreadCount,
		},
		"related": fiber.Map{
			"users":     related.UserList(),
			"tracks":    related.TrackList(),
			"playlists": related.PlaylistList(),
		},
	})

}

// collectNotificationRelatedIds extracts user/track/playlist IDs from a single
// raw (pre-hashify) notification action's data so the caller can batch-load
// the related entities in one shot. Field names mirror the Python
// extend_notification.py mapping; *_item_id and content_id fields are
// polymorphic and disambiguated by the sibling type field.
func collectNotificationRelatedIds(action json.RawMessage, userIds, trackIds, playlistIds *[]int32) {
	appendInt := func(target *[]int32, val gjson.Result) {
		if val.Exists() && val.Type == gjson.Number {
			*target = append(*target, int32(val.Int()))
		}
	}

	for _, path := range []string{
		"data.user_id",
		"data.follower_user_id",
		"data.followee_user_id",
		"data.comment_user_id",
		"data.entity_user_id",
		"data.reacter_user_id",
		"data.sender_user_id",
		"data.receiver_user_id",
		"data.dethroned_user_id",
		"data.grantee_user_id",
		"data.inviter_user_id",
		"data.collaborator_user_id",
		"data.tastemaker_user_id",
		"data.tastemaker_item_owner_id",
		"data.track_owner_id",
		"data.parent_track_owner_id",
		"data.playlist_owner_id",
		"data.buyer_user_id",
		"data.seller_user_id",
	} {
		appendInt(userIds, gjson.GetBytes(action, path))
	}

	appendInt(trackIds, gjson.GetBytes(action, "data.track_id"))
	appendInt(trackIds, gjson.GetBytes(action, "data.parent_track_id"))
	appendInt(playlistIds, gjson.GetBytes(action, "data.playlist_id"))

	// Polymorphic fields: split by sibling type discriminator.
	itemType := strings.ToLower(gjson.GetBytes(action, "data.type").String())
	for _, path := range []string{
		"data.repost_item_id",
		"data.save_item_id",
		"data.repost_of_repost_item_id",
		"data.save_of_repost_item_id",
	} {
		val := gjson.GetBytes(action, path)
		if !val.Exists() || val.Type != gjson.Number {
			continue
		}
		if itemType == "track" {
			*trackIds = append(*trackIds, int32(val.Int()))
		} else if itemType == "playlist" || itemType == "album" {
			*playlistIds = append(*playlistIds, int32(val.Int()))
		}
	}

	if val := gjson.GetBytes(action, "data.tastemaker_item_id"); val.Exists() && val.Type == gjson.Number {
		switch strings.ToLower(gjson.GetBytes(action, "data.tastemaker_item_type").String()) {
		case "track":
			*trackIds = append(*trackIds, int32(val.Int()))
		case "playlist", "album":
			*playlistIds = append(*playlistIds, int32(val.Int()))
		}
	}

	if val := gjson.GetBytes(action, "data.content_id"); val.Exists() && val.Type == gjson.Number {
		switch strings.ToLower(gjson.GetBytes(action, "data.content_type").String()) {
		case "track":
			*trackIds = append(*trackIds, int32(val.Int()))
		case "playlist", "album":
			*playlistIds = append(*playlistIds, int32(val.Int()))
		}
	}

	// Comment notifications: entity_id is a track when entity_type is Track.
	if val := gjson.GetBytes(action, "data.entity_id"); val.Exists() && val.Type == gjson.Number {
		if strings.EqualFold(gjson.GetBytes(action, "data.entity_type").String(), "track") {
			*trackIds = append(*trackIds, int32(val.Int()))
		}
	}
}
