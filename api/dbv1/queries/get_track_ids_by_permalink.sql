-- name: GetTrackIdsByPermalink :many
WITH lower_handles AS (
  SELECT LOWER(h) AS handle
  FROM unnest(@handles::text[]) AS h
),
lower_permalinks AS (
  SELECT LOWER(p) AS permalink
  FROM unnest(@permalinks::text[]) AS p
)
SELECT tr.track_id
FROM track_routes tr
JOIN users u ON u.user_id = tr.owner_id
JOIN lower_handles lh
  ON u.handle_lc = lh.handle
WHERE tr.slug = ANY(@slugs::text[])
  -- in case of conflicts across usAers
  AND CONCAT('/', u.handle_lc, '/', LOWER(tr.slug)) = ANY(
    SELECT permalink FROM lower_permalinks
  );
