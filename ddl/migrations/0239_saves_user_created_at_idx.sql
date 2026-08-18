-- Supports /v1/users/:id/suggested-follows, which takes a bounded slice of the
-- user's most recent favorites:
--
--   WHERE user_id = ?
--     AND is_delete = false
--   ORDER BY created_at DESC
--   LIMIT ?
--
-- No existing index can serve that ordering. saves_user_idx orders by save_type
-- and save_item_id before created_at, and saves_user_track_current_blocknumber_idx
-- orders by blocknumber and is partial on save_type = 'track' (the endpoint also
-- reads album/playlist saves, so it can't use a track-only index). Postgres
-- therefore reads every one of the user's saves and top-N sorts them, which for
-- a user with a large library dominates the request.
--
-- Mirrors reposts_user_created_at_active_idx (migration 0223), which already
-- solves exactly this for the reposts half of the same query.
CREATE INDEX CONCURRENTLY IF NOT EXISTS saves_user_created_at_active_idx
    ON public.saves USING btree (user_id, created_at DESC)
    INCLUDE (save_type, save_item_id)
    WHERE is_delete = false;
