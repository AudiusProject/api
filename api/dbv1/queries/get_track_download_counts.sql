-- name: GetTrackDownloadCounts :many
SELECT
  t.track_id,
  (
    SELECT count(*)::bigint
    FROM track_downloads d
    WHERE (t.stem_of IS NOT NULL
           AND d.parent_track_id = (t.stem_of->>'parent_track_id')::int
           AND d.track_id = t.track_id)
       OR (t.stem_of IS NULL AND d.parent_track_id = t.track_id)
  ) AS download_count
FROM tracks t
WHERE t.track_id = ANY(@track_ids::int[])
  AND t.is_current = true
  AND t.is_delete = false;
