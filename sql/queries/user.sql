-- name: GetUserByHandle :one
SELECT
  u.user_id,
  handle,
  wallet,
  name,
  bio,
  location,
  follower_count,
  track_count,

  (
    SELECT count(*) > 0
    FROM follows
    WHERE @my_id > 0
      AND follower_user_id = @my_id
      AND followee_user_id = u.user_id
      AND is_delete = false
  ) AS does_current_user_follow,

  (
    SELECT count(*) > 0
    FROM follows
    WHERE @my_id > 0
      AND followee_user_id = @my_id
      AND follower_user_id = u.user_id
      AND is_delete = false
  ) AS does_follow_current_user

FROM users u
JOIN aggregate_user using (user_id)
WHERE handle_lc = lower(@handle)
  AND is_deactivated = false
LIMIT 1;
