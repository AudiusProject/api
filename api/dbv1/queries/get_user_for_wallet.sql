-- name: GetUserForWallet :one
SELECT user_id FROM users
WHERE
    wallet = lower(@wallet)
    AND is_current = true
-- Prefer an active completed profile over deactivated accounts or unfinished signups.
ORDER BY is_deactivated ASC, handle IS NOT NULL DESC, created_at ASC, user_id ASC
LIMIT 1;
