-- name: GetUserForWallet :one
SELECT user_id FROM users
WHERE
    wallet = lower(@wallet)
    AND is_current = true
ORDER BY handle IS NOT NULL, created_at ASC
LIMIT 1;