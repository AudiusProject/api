-- Partial index covering the slice of track_trending_scores that the
-- /v1/users/{id}/feed/for-you "trending" and "underground" candidate
-- sources hit on every request:
--
--   WHERE type = 'TRACKS'
--     AND version = 'pnagD'
--     AND time_range = 'week'
--     AND (genre IS NULL OR genre = '')
--   ORDER BY score DESC, track_id
--   LIMIT 100/50
--
-- Without this index EXPLAIN shows a fixed ~12s scan of the full
-- track_trending_scores table for every request, regardless of caller.
-- This was the single biggest non-similar-artists cost in the original
-- For You endpoint (#807).
--
-- Size budget: the matching slice is on the order of a few thousand
-- rows (one row per current week-trending track), so the index is small.
--
-- NOTE: intentionally NOT wrapped in BEGIN/COMMIT so that
-- CREATE INDEX CONCURRENTLY can run without holding an ACCESS EXCLUSIVE
-- lock on track_trending_scores. IF NOT EXISTS makes the migration
-- idempotent.

create index concurrently if not exists idx_track_trending_scores_for_you
    on track_trending_scores (score desc, track_id)
    where type = 'TRACKS'
      and version = 'pnagD'
      and time_range = 'week'
      and (genre is null or genre = '');

comment on index idx_track_trending_scores_for_you is
    'Partial index for the For You feed trending/underground candidate sources; replaces a ~12s full-table scan with a small index seek.';
