-- name: GetPlaylistIdsByPermalink :many
SELECT playlist_id
FROM playlist_routes
JOIN users ON users.user_id = playlist_routes.owner_id
WHERE handle_lc = ANY(@handles::text[])
    AND slug = ANY(@slugs::text[])
    AND (
        CONCAT(handle_lc, '/playlist/', slug) = ANY(@permalinks::text[]) -- in case of conflicts across users
        OR CONCAT(handle_lc, '/album/', slug) = ANY(@permalinks::text[]) -- in case of conflicts across users
    );
