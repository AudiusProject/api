-- name: GetUserTrackDownloadCountTotal :one
-- Total download count for all tracks (and stems) owned by the user.
-- Each download event is counted once.
SELECT count(*)::bigint AS total_download_count
FROM track_downloads d
WHERE EXISTS (
  SELECT 1
  FROM tracks t
  WHERE t.owner_id = @user_id
    AND t.is_current = true
    AND t.is_delete = false
    AND (
      (t.stem_of IS NULL AND d.parent_track_id = t.track_id)
      OR (t.stem_of IS NOT NULL
          AND (t.stem_of->>'parent_track_id')::int = d.parent_track_id
          AND d.track_id = t.track_id)
    )
);
