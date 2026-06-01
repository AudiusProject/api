-- Promote the track_id indexes on the trending materialized views to UNIQUE.
--
-- TrendingJob refreshes aggregate_interval_plays and trending_params on every
-- run. Today it uses a plain `REFRESH MATERIALIZED VIEW`, which takes an
-- ACCESS EXCLUSIVE lock on the matview for the duration of the (multi-minute,
-- 64GB-plays-scanning) rebuild — blocking every reader, including the
-- block-indexing loop's downstream queries.
--
-- `REFRESH MATERIALIZED VIEW CONCURRENTLY` avoids that lock, but Postgres
-- requires a UNIQUE index with no WHERE clause on the matview to use it. Both
-- matviews are exactly one row per track (verified in prod: row count ==
-- COUNT(DISTINCT track_id) for each), so a unique index on track_id is valid.
-- We replace the existing NON-unique track_id indexes in place rather than add
-- a second redundant index on the same column.
--
-- NOTE: intentionally NOT wrapped in BEGIN/COMMIT. Each statement autocommits
-- so the brief ACCESS EXCLUSIVE taken by DROP/CREATE INDEX is held only for the
-- (sub-second, ~1.4M-row) index build, not the whole file. IF NOT EXISTS keeps
-- the CREATE idempotent if the migration is re-applied.

DROP INDEX IF EXISTS public.interval_play_track_id_idx;
CREATE UNIQUE INDEX IF NOT EXISTS interval_play_track_id_idx
    ON public.aggregate_interval_plays USING btree (track_id);

DROP INDEX IF EXISTS public.trending_params_track_id_idx;
CREATE UNIQUE INDEX IF NOT EXISTS trending_params_track_id_idx
    ON public.trending_params USING btree (track_id);
