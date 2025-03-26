-- name: GetUserByHandle :one
SELECT
  u.user_id,
  handle,
  wallet,
  name,
  bio,
  location,
  follower_count,
  track_count
FROM users u
JOIN aggregate_user using (user_id)
WHERE handle_lc = lower($1) LIMIT 1;
