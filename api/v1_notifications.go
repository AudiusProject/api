package api

import (
	"encoding/json"
	"slices"
	"strings"

	"bridgerton.audius.co/trashid"
	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

type GetNotificationsQueryParams struct {
	// Note that when limit is 0, we return 20 items to calculate unread count
	Limit     int      `query:"limit" default:"20" validate:"min=0,max=100"`
	Types     []string `query:"types" validate:"dive,oneof=announcement follow repost save remix cosign create tip_receive tip_send challenge_reward repost_of_repost save_of_repost tastemaker reaction supporter_dethroned supporter_rank_up supporting_rank_up milestone track_milestone track_added_to_playlist playlist_milestone tier_change trending trending_playlist trending_underground usdc_purchase_buyer usdc_purchase_seller track_added_to_purchased_album request_manager approve_manager_request claimable_reward comment comment_thread comment_mention comment_reaction listen_streak_reminder fan_remix_contest_started fan_remix_contest_ended fan_remix_contest_ending_soon fan_remix_contest_winners_selected artist_remix_contest_ended artist_remix_contest_ending_soon artist_remix_contest_submissions"`
	GroupID   string   `query:"group_id" validate:"omitempty"`
	Timestamp float64  `query:"timestamp" validate:"omitempty,min=0"`
}

func (app *ApiServer) v1Notifications(c *fiber.Ctx) error {
	params := GetNotificationsQueryParams{}
	if err := app.ParseAndValidateQueryParams(c, &params); err != nil {
		return err
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
user_created_at as (
  SELECT
    created_at
  FROM
    users
  WHERE
    user_id =  @user_id
  AND is_current
)
SELECT
	n.type,
	n.group_id AS group_id,
	json_agg(
		json_build_object(
			'type', type,
			'specifier', specifier,
			'timestamp', EXTRACT(EPOCH FROM timestamp),
			'data', data
		)
		ORDER BY timestamp DESC
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
    notification n
LEFT JOIN user_seen ON
  user_seen.seen_at >= n.timestamp AND user_seen.prev_seen_at < n.timestamp
WHERE
  ((ARRAY[@user_id] && n.user_ids) OR (n.type = 'announcement' AND n.timestamp > (SELECT created_at FROM user_created_at)))
  AND (n.type = ANY(@types) OR @types IS NULL)
  AND (
    (@timestamp_offset = 0 AND @group_id_offset = '') OR
    (@timestamp_offset = 0 AND @group_id_offset != '' AND n.group_id < @group_id_offset) OR
    (@timestamp_offset > 0 AND n.timestamp < to_timestamp(@timestamp_offset)) OR
    (
        @group_id_offset != '' AND @timestamp_offset > 0 AND
        (n.timestamp = to_timestamp(@timestamp_offset) AND n.group_id < @group_id_offset)
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
	type GetNotifsRow struct {
		Type    string            `json:"type"`
		GroupID string            `json:"group_id"`
		Actions []json.RawMessage `json:"actions"`
		IsSeen  bool              `json:"is_seen"`
		SeenAt  interface{}       `json:"seen_at"`
	}

	rows, err := app.pool.Query(c.Context(), sql, pgx.NamedArgs{
		"user_id":          userId,
		"limit":            params.Limit,
		"types":            params.Types,
		"group_id_offset":  params.GroupID,
		"timestamp_offset": params.Timestamp,
	})
	if err != nil {
		return err
	}

	notifs, err := pgx.CollectRows(rows, pgx.RowToAddrOfStructByNameLax[GetNotifsRow])
	if err != nil {
		return err
	}

	unreadCount := 0
	for _, notif := range notifs {

		slices.SortFunc(notif.Actions, func(a, b json.RawMessage) int {
			specA := gjson.GetBytes(a, "specifier").String()
			specB := gjson.GetBytes(b, "specifier").String()
			return strings.Compare(specA, specB)
		})

		// each row from notification table has `actions`
		// which is a jsonb field that is an array of objects.
		// we need to hash encode all id fields (HashifyJson)
		// and do some additional transforms.
		// see extend_notification.py for details
		for idx, action := range notif.Actions {
			action = trashid.HashifyJson(action)

			// type: lowercase
			if val := gjson.GetBytes(action, "data.type"); val.Exists() {
				action, _ = sjson.SetBytes(action, "data.type", strings.ToLower(val.String()))
			}

			// for playlist milestones: is_album: default to false
			if strings.HasPrefix(notif.GroupID, "milestone:PLAYLIST_") {
				isAlbum := gjson.GetBytes(action, "data.is_album").Bool()
				action, _ = sjson.SetBytes(action, "data.is_album", isAlbum)
			}

			// amount + tip_amount: to_wei_string
			for _, fieldPath := range []string{"data.amount", "data.tip_amount"} {
				if val := gjson.GetBytes(action, fieldPath); val.Exists() {
					action, _ = sjson.SetBytes(action, fieldPath, val.String()+"0000000000")
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

	return c.JSON(fiber.Map{
		"data": fiber.Map{
			"notifications": notifs,
			"unread_count":  unreadCount,
		},
	})

}
