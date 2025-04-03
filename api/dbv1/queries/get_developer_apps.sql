-- name: GetDeveloperApps :many
SELECT
  address,
  user_id,
  name,
  description,
  image_url
FROM developer_apps
WHERE 
  (user_id = @user_id OR address = @address)
  AND is_current = true
  AND is_delete = false
ORDER BY created_at DESC;

-- name: GetDeveloperAppsWithGrants :many
SELECT
  developer_apps.address,
  developer_apps.name,
  developer_apps.description,
  developer_apps.image_url,
  grants.user_id AS grantor_user_id,
  grants.created_at AS grant_created_at,
  grants.updated_at AS grant_updated_at
FROM developer_apps
LEFT JOIN grants ON grants.grantee_address = developer_apps.address
WHERE
  (grants.user_id = @user_id OR developer_apps.address = @address)
  AND grants.is_revoked = false
  AND grants.is_current = true
  AND developer_apps.is_current = true
  AND developer_apps.is_delete = false
ORDER BY grants.updated_at ASC;
