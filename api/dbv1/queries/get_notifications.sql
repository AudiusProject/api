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
    )::jsonb as actions,
    CASE
      WHEN user_seen.seen_at is not NULL THEN now()::timestamp != user_seen.seen_at
      ELSE EXISTS(SELECT 1 from notification_seen ns where ns.user_id = @user_id)
    END as is_seen,
    CASE
      WHEN user_seen.seen_at is not NULL THEN EXTRACT(EPOCH FROM user_seen.seen_at)
      ELSE (
        SELECT EXTRACT(EPOCH FROM seen_at)
        from notification_seen ns
        WHERE ns.user_id = @user_id
        ORDER BY seen_at ASC
        limit 1
      )
    END as seen_at
FROM
    notification n
LEFT JOIN user_seen on
  user_seen.seen_at >= n.timestamp and user_seen.prev_seen_at < n.timestamp
WHERE
  ((ARRAY[@user_id] && n.user_ids) OR (n.type = 'announcement' AND n.timestamp > (SELECT created_at FROM user_created_at)))
  AND n.type IN (
    'repost',
    'save',
    'follow',
    'tip_send',
    'tip_receive',
    'milestone',
    'supporter_rank_up',
    'supporting_rank_up',
    'challenge_reward',
    'tier_change',
    'create',
    'remix',
    'cosign',
    'trending',
    'supporter_dethroned',
    -- NotificationType.ANNOUNCEMENT,
    'reaction',
    'track_added_to_playlist'
  )
GROUP BY
  n.type, n.group_id, user_seen.seen_at, user_seen.prev_seen_at
ORDER BY
  user_seen.seen_at desc NULLS LAST,
  max(n.timestamp) desc,
  n.group_id desc
limit @lim::int
;
