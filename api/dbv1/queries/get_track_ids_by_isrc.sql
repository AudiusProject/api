-- name: GetTrackIdsByISRC :many
-- Match ISRC ignoring dashes/whitespace/case so a stored value like
-- "US-ANG-21-03742" matches a query of "USANG2103742" and vice versa.
-- Inputs in @isrcs must already be normalized (uppercased, non-alphanumerics stripped).
SELECT track_id
FROM tracks
WHERE regexp_replace(upper(isrc), '[^A-Z0-9]', '', 'g') = ANY(@isrcs::text[])
  AND access_authorities IS NULL;
