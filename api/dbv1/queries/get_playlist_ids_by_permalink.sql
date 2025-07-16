-- name: GetPlaylistIdsByPermalink :many
WITH lower_handles AS (
  SELECT LOWER(h) AS handle
  FROM unnest(@handles::text[]) AS h
),
lower_permalinks AS (
  SELECT LOWER(p) AS permalink
  FROM unnest(@permalinks::text[]) AS p
)
SELECT pr.playlist_id
FROM playlist_routes pr
JOIN users u ON u.user_id = pr.owner_id
JOIN lower_handles lh
  ON u.handle_lc = lh.handle
WHERE pr.slug = ANY(@slugs::text[])
  -- in case of conflicts across users
  AND (
    CONCAT('/', u.handle_lc, '/playlist/', LOWER(pr.slug)) = ANY(
      SELECT permalink FROM lower_permalinks
    )
    OR CONCAT('/', u.handle_lc, '/album/', LOWER(pr.slug)) = ANY(
      SELECT permalink FROM lower_permalinks
    )
  );
