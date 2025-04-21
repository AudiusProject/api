-- name: GetExtendedAccountFields :one
SELECT
  playlist_library,
  track_save_count
FROM users
JOIN aggregate_user ON users.user_id = aggregate_user.user_id
WHERE users.user_id = @user_id;