-- name: GetAccountPlaylists :many
WITH playlist_ids AS (
    SELECT save_item_id as id
    FROM saves
    WHERE saves.user_id = @user_id
      AND is_delete = FALSE
      AND (save_type = 'playlist' OR save_type = 'album')
    UNION
    SELECT p.playlist_id AS id
    FROM playlists p
    WHERE p.is_delete = FALSE
      AND p.playlist_owner_id = @user_id
)
SELECT
    p.playlist_id as id,
    p.playlist_id,
    p.is_album,
    -- p.permalink // TODO
    p.playlist_name as name,

    json_build_object (
      'id', u.user_id,
      'handle', u.handle,
      'is_deactivated', u.is_deactivated
    ) as user,

    p.created_at
FROM playlists p
JOIN users u ON p.playlist_owner_id = u.user_id
WHERE p.is_delete = false
  AND p.playlist_id IN (SELECT id FROM playlist_ids)
ORDER BY p.created_at DESC, p.playlist_id ASC;
