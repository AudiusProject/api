-- name: GetUserForHandle :one
SELECT user_id FROM users
WHERE
    handle_lc = lower(@handle)
    AND is_current = true
ORDER BY created_at ASC
LIMIT 1;
