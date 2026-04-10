-- name: GetDeveloperApps :many
SELECT
  da.address,
  da.user_id,
  da.name,
  da.description,
  da.image_url,
  COALESCE(ARRAY_AGG(oau.redirect_uri ORDER BY oau.id) FILTER (WHERE oau.redirect_uri IS NOT NULL), ARRAY[]::text[])::text[] AS redirect_uris
FROM developer_apps da
LEFT JOIN oauth_redirect_uris oau ON oau.client_id = da.address
WHERE
  (da.user_id = @user_id OR lower(da.address) = lower(@address))
  AND da.is_current = true
  AND da.is_delete = false
GROUP BY da.address, da.user_id, da.name, da.description, da.image_url, da.created_at
ORDER BY da.created_at DESC;

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
  (grants.user_id = @user_id OR lower(developer_apps.address) = lower(@address))
  AND grants.is_revoked = false
  AND grants.is_current = true
  AND developer_apps.is_current = true
  AND developer_apps.is_delete = false
ORDER BY grants.updated_at ASC;
