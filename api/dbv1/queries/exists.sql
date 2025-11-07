-- name: UserExists :one
SELECT EXISTS(
  SELECT 1
  FROM users
  WHERE 
    wallet = lower(@wallet)
    AND is_current = true
    AND is_deactivated = false
);

-- name: DeveloperAppExists :one
SELECT EXISTS(
  SELECT 1
  FROM developer_apps
  WHERE 
    address = @address
    AND is_current = true
    AND is_delete = false
);

-- name: GrantExists :one
SELECT EXISTS(
  SELECT 1
  FROM grants
  WHERE 
    user_id = @user_id
    AND grantee_address = @grantee_address
    AND is_current = true
    AND is_revoked = false
);

