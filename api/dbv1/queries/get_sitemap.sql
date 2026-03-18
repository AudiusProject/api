-- name: GetSitemapTrackCount :one
SELECT count(*) as count
FROM track_routes tr
JOIN tracks t ON tr.track_id = t.track_id
JOIN users u ON tr.owner_id = u.user_id
JOIN aggregate_user au ON u.user_id = au.user_id
WHERE t.is_current = true
  AND t.stem_of IS NULL
  AND t.is_unlisted = false
  AND t.is_available = true
  AND t.is_delete = false
  AND u.is_current = true
  AND tr.is_current = true
  AND au.follower_count >= 10;

-- name: GetSitemapTrackSlugs :many
SELECT u.handle, tr.slug
FROM track_routes tr
JOIN tracks t ON tr.track_id = t.track_id
JOIN users u ON tr.owner_id = u.user_id
JOIN aggregate_user au ON u.user_id = au.user_id
WHERE t.is_current = true
  AND t.stem_of IS NULL
  AND t.is_unlisted = false
  AND t.is_available = true
  AND t.is_delete = false
  AND u.is_current = true
  AND tr.is_current = true
  AND au.follower_count >= 10
ORDER BY t.track_id ASC
LIMIT $1 OFFSET $2;

-- name: GetSitemapPlaylistCount :one
SELECT count(*) as count
FROM playlist_routes pr
JOIN playlists p ON pr.playlist_id = p.playlist_id
JOIN users u ON pr.owner_id = u.user_id
JOIN aggregate_user au ON u.user_id = au.user_id
WHERE u.is_current = true
  AND pr.is_current = true
  AND p.is_current = true
  AND p.is_private = false
  AND p.is_delete = false
  AND au.follower_count >= 10;

-- name: GetSitemapPlaylistSlugs :many
SELECT u.handle, pr.slug, p.is_album
FROM playlist_routes pr
JOIN playlists p ON pr.playlist_id = p.playlist_id
JOIN users u ON pr.owner_id = u.user_id
JOIN aggregate_user au ON u.user_id = au.user_id
WHERE u.is_current = true
  AND pr.is_current = true
  AND p.is_current = true
  AND p.is_private = false
  AND p.is_delete = false
  AND au.follower_count >= 10
ORDER BY p.playlist_id ASC
LIMIT $1 OFFSET $2;

-- name: GetSitemapUserCount :one
SELECT count(*) as count
FROM users u
JOIN aggregate_user au ON u.user_id = au.user_id
WHERE u.is_current = true
  AND u.is_deactivated = false
  AND u.handle_lc IS NOT NULL
  AND u.is_available = true
  AND au.follower_count >= 10;

-- name: GetSitemapUserSlugs :many
SELECT u.handle
FROM users u
JOIN aggregate_user au ON u.user_id = au.user_id
WHERE u.is_current = true
  AND u.is_deactivated = false
  AND u.handle_lc IS NOT NULL
  AND u.is_available = true
  AND au.follower_count >= 10
ORDER BY u.user_id ASC
LIMIT $1 OFFSET $2;
