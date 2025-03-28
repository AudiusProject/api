-- name: GetTracks :many
SELECT
  t.track_id,
  owner_id,
  title,
  is_unlisted,

  repost_count,
  save_count as favorite_count,

  (
    SELECT count(*) > 0
    FROM reposts
    WHERE @my_id > 0
      AND user_id = @my_id
      AND repost_type = 'track'
      AND repost_item_id = t.track_id
      AND is_delete = false
  ) AS has_current_user_reposted

FROM tracks t
JOIN aggregate_track using (track_id)
WHERE is_available = true
  AND (is_unlisted = false OR owner_id = @my_id)
  AND (
    t.track_id = @track_id
    OR t.owner_id = @owner_id
  )
ORDER BY t.track_id
;
