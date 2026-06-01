-- Promote the track_id indexes on the trending matviews to UNIQUE so
-- TrendingJob can use REFRESH MATERIALIZED VIEW CONCURRENTLY (which avoids the
-- ACCESS EXCLUSIVE lock a plain refresh takes). CONCURRENTLY requires a UNIQUE
-- index with no WHERE clause; both matviews are one row per track (verified in
-- prod: row count == COUNT(DISTINCT track_id)), so this is valid. We replace
-- the existing non-unique indexes in place.
--
-- Intentionally NOT wrapped in BEGIN/COMMIT: each statement autocommits so the
-- brief ACCESS EXCLUSIVE from DROP/CREATE INDEX is held only for the index
-- build. IF NOT EXISTS keeps re-application idempotent.

DROP INDEX IF EXISTS public.interval_play_track_id_idx;
CREATE UNIQUE INDEX IF NOT EXISTS interval_play_track_id_idx
    ON public.aggregate_interval_plays USING btree (track_id);

DROP INDEX IF EXISTS public.trending_params_track_id_idx;
CREATE UNIQUE INDEX IF NOT EXISTS trending_params_track_id_idx
    ON public.trending_params USING btree (track_id);
