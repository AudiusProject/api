package api

import (
	"encoding/json"
	"slices"

	"bridgerton.audius.co/trashid"
	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"
)

func (app *ApiServer) v1Notifications(c *fiber.Ctx) error {

	sql := `
-- name: GetNotifs :many
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
    n.group_id as group_id,
    json_agg(
      json_build_object(
        'type', type,
        'specifier', specifier::int,
        'timestamp', EXTRACT(EPOCH FROM timestamp),
        'data', data
      )
      ORDER BY timestamp DESC
    )::jsonb as actions,
    CASE
      WHEN user_seen.seen_at is not NULL THEN now()::timestamp != user_seen.seen_at
      ELSE EXISTS(SELECT 1 from notification_seen ns where ns.user_id = @user_id)
    END::boolean as is_seen,

    CASE
      WHEN user_seen.seen_at != now()::timestamp THEN EXTRACT(EPOCH FROM user_seen.seen_at)
      ELSE null
    END AS seen_at

FROM
    notification n
LEFT JOIN user_seen on
  user_seen.seen_at >= n.timestamp and user_seen.prev_seen_at < n.timestamp
WHERE
  ((ARRAY[@user_id] && n.user_ids) OR (n.type = 'announcement' AND n.timestamp > (SELECT created_at FROM user_created_at)))
  AND n.type = ANY(@valid_types)
GROUP BY
  n.type, n.group_id, user_seen.seen_at, user_seen.prev_seen_at
ORDER BY
  user_seen.seen_at desc NULLS LAST,
  max(n.timestamp) desc,
  n.group_id desc
limit @limit::int
;
`

	// default types are always enabled
	validTypes := []string{
		"repost",
		"save",
		"follow",
		"tip_send",
		"tip_receive",
		"milestone",
		"supporter_rank_up",
		"supporting_rank_up",
		"challenge_reward",
		"tier_change",
		"create",
		"remix",
		"cosign",
		"trending",
		"supporter_dethroned",
		"reaction",
		"track_added_to_playlist",
	}

	// add optional valid_types
	for _, t := range queryMulti(c, "valid_types") {
		if !slices.Contains(validTypes, t) {
			validTypes = append(validTypes, t)
		}
	}

	userId := app.getUserId(c)
	limit := c.QueryInt("limit", 20)

	// python returns 20 items when limit=0
	// and client relies on this for showing unread count
	if limit == 0 {
		limit = 20
	}

	type GetNotifsRow struct {
		Type    string          `json:"type"`
		GroupID string          `json:"group_id"`
		Actions json.RawMessage `json:"actions"`
		IsSeen  bool            `json:"is_seen"`
		SeenAt  interface{}     `json:"seen_at"`
	}

	rows, err := app.pool.Query(c.Context(), sql, pgx.NamedArgs{
		"user_id":     userId,
		"limit":       limit,
		"valid_types": validTypes,
	})
	if err != nil {
		return err
	}

	notifs, err := pgx.CollectRows(rows, pgx.RowToStructByNameLax[GetNotifsRow])
	if err != nil {
		return err
	}

	unreadCount := 0
	for idx, notif := range notifs {
		notif.Actions = trashid.HashifyJson(notif.Actions)
		notifs[idx] = notif
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
