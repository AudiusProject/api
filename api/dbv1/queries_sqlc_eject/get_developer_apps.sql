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


