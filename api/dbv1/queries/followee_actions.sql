-- name: FolloweeActions :many
WITH my_follows AS (
  SELECT
    followee_user_id as user_id,
    follower_count
  FROM follows
  JOIN aggregate_user ON followee_user_id = user_id
  WHERE @my_id > 0
    AND follower_user_id = @my_id
    AND follows.is_delete = false
  ORDER BY follower_count DESC
  LIMIT 5000
),

followee_reposts AS (
  SELECT
    'repost' as verb,
    reposts.user_id,
    repost_item_id as item_id,
    reposts.created_at,
    ROW_NUMBER() OVER (PARTITION BY repost_item_id ORDER BY created_at DESC) AS row_index
  FROM reposts
  JOIN my_follows USING (user_id)
  WHERE repost_item_id = ANY(@ids::int[])
    AND repost_type = 'track'
    AND reposts.is_delete = false
  ORDER BY follower_count DESC
),

followee_saves AS (
  SELECT
    'save' as verb,
    saves.user_id,
    save_item_id as item_id,
    saves.created_at,
    ROW_NUMBER() OVER (PARTITION BY save_item_id ORDER BY created_at DESC) AS row_index
  FROM saves
  JOIN my_follows USING (user_id)
  WHERE save_item_id = ANY(@ids::int[])
    AND save_type = 'track'
    AND saves.is_delete = false
  ORDER BY follower_count DESC
)

SELECT * FROM followee_reposts WHERE row_index < 6
UNION ALL
SELECT * FROM followee_saves WHERE row_index < 6
;
