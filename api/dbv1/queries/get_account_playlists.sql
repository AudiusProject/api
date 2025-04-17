-- name: GetAccountPlaylists :many
WITH saved_playlists AS (
    SELECT save_item_id
    FROM saves
    WHERE user_id = @user_id
      AND is_current = TRUE
      AND is_delete = FALSE
      AND (save_type = 'playlist' OR save_type = 'album')
)
SELECT * FROM (
    SELECT DISTINCT ON (p.playlist_id)
        p.playlist_id,
        p.is_album,
        -- p.permalink // TODO
        p.playlist_name,
        u.user_id,
        u.handle,
        u.is_deactivated,
        p.created_at
    FROM playlists p
    JOIN users u ON p.playlist_owner_id = u.user_id
    WHERE (
        -- Owned playlists
        (p.is_current = TRUE
         AND p.is_delete = FALSE
         AND p.playlist_owner_id = @user_id)

        OR

        -- Saved playlists
        (p.is_current = TRUE
         AND p.is_delete = FALSE
         AND p.playlist_id IN (SELECT save_item_id FROM saved_playlists))
    )
    ORDER BY p.playlist_id
) subquery
ORDER BY created_at DESC;
