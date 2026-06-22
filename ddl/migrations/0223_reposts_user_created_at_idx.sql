-- Supports /v1/users/:id/reposts pagination:
--
--   WHERE user_id = ?
--     AND is_delete = false
--   ORDER BY created_at DESC
--
-- The existing reposts_user_idx is ordered by repost_type and repost_item_id
-- before created_at. For users whose reposts are not recent globally, Postgres
-- can choose reposts_new_created_at_idx and scan millions of unrelated rows to
-- find the user's newest reposts.
CREATE INDEX CONCURRENTLY IF NOT EXISTS reposts_user_created_at_active_idx
    ON public.reposts USING btree (user_id, created_at DESC)
    INCLUDE (repost_type, repost_item_id)
    WHERE is_delete = false;
