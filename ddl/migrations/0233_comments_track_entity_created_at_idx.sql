-- Supports track comment listing and counts:
--
--   WHERE entity_id = ?
--     AND entity_type = 'Track'
--     AND is_delete = false
--   ORDER BY created_at DESC
--
-- These endpoints otherwise scan the full comments table for each track. The
-- included columns cover the comment fields used before moderation joins and
-- keep this partial index small enough for the serving read path.
CREATE INDEX CONCURRENTLY IF NOT EXISTS comments_track_entity_created_at_idx
    ON public.comments USING btree (entity_id, created_at DESC)
    INCLUDE (comment_id, user_id)
    WHERE entity_type = 'Track'
      AND is_delete = false;
