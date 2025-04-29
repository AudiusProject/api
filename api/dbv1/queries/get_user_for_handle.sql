-- name: GetUserForHandle :one
SELECT user_id FROM users
WHERE
    handle_lc = lower(@handle)
ORDER BY created_at ASC
LIMIT 1;
