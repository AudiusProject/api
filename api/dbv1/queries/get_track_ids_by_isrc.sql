-- name: GetTrackIdsByISRC :many
SELECT track_id
FROM tracks
WHERE isrc = ANY(@isrcs::text[]);