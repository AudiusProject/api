-- name: GetTrackIdsByISRC :many
SELECT track_id
FROM tracks
WHERE isrc = ANY(@isrcs::text[])
  AND is_current = true
  AND access_authorities IS NULL;