-- Supports /v1/users/:id/favorites pagination:
--
--   WHERE user_id = ?
--     AND is_delete = false
--     AND is_current = true
--     AND save_type = 'track'
--   ORDER BY blocknumber, save_item_id DESC
--
-- The existing saves_user_idx is ordered by save_item_id before is_delete and
-- cannot satisfy the endpoint's blocknumber ordering, which showed up in prod
-- pg_stat_statements with high total temp I/O.
CREATE INDEX CONCURRENTLY IF NOT EXISTS saves_user_track_current_blocknumber_idx
    ON public.saves USING btree (user_id, blocknumber, save_item_id DESC)
    INCLUDE (created_at)
    WHERE save_type = 'track'
      AND is_current = true
      AND is_delete = false;
