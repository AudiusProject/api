-- name: GetPlaylists :many
SELECT
  p.playlist_id,
  p.playlist_name,
  p.playlist_owner_id,
  p.playlist_contents,

  (
    SELECT count(*) > 0
    FROM reposts
    WHERE @my_id > 0
      AND user_id = @my_id
      AND repost_type != 'track'
      AND repost_item_id = p.playlist_id
      AND is_delete = false
  ) AS has_current_user_reposted,

  (
    SELECT count(*) > 0
    FROM saves
    WHERE @my_id > 0
      AND user_id = @my_id
      AND save_type != 'track'
      AND save_item_id = p.playlist_id
      AND is_delete = false
  ) AS has_current_user_saved

FROM playlists p
WHERE is_delete = false
  and playlist_id = ANY(@ids::int[])
;
