-- name: GetGrantsForUserId :many
SELECT
  g.user_id,
  g.grantee_address,
  g.is_revoked,
  g.is_approved,
  g.created_at,
  g.updated_at,
  u.user_id as grantee_user_id
FROM grants g
JOIN users u ON u.wallet = g.grantee_address
WHERE g.user_id = @user_id::int
  AND g.is_revoked = @is_revoked
  AND g.is_current = true
  AND sqlc.narg('is_approved')::boolean IS NULL OR g.is_approved = sqlc.narg('is_approved')
ORDER BY g.created_at DESC;

-- name: GetGrantsForGranteeAddress :many
SELECT
  g.user_id,
  g.grantee_address,
  g.is_revoked,
  g.is_approved,
  g.created_at,
  g.updated_at,
  u.user_id as grantee_user_id
FROM grants g
JOIN users u ON u.wallet = g.grantee_address
WHERE g.grantee_address = @grantee_address
  AND g.is_current = true
  AND g.is_revoked = @is_revoked
  AND sqlc.narg('is_approved')::boolean IS NULL OR g.is_approved = sqlc.narg('is_approved')
ORDER BY g.created_at DESC;