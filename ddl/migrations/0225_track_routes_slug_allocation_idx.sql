-- Supports ETL track route allocation:
--
--   SELECT MAX(collision_id)
--   FROM track_routes
--   WHERE owner_id = ?
--     AND title_slug = ?
--
-- The primary key covers (owner_id, slug), but not title_slug. Track creates
-- run this lookup before inserting each track_routes row, so give Postgres a
-- narrow index for the slug-allocation path.
CREATE INDEX CONCURRENTLY IF NOT EXISTS track_routes_owner_title_slug_collision_idx
    ON public.track_routes USING btree (owner_id, title_slug, collision_id DESC);
