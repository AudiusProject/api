-- name: GetExtendedAccountFields :one
SELECT
  track_save_count
FROM aggregate_user
WHERE user_id = @user_id;
