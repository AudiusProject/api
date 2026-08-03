-- Supports /v1/users/:id/comments pagination:
--
--   WHERE user_id = ?
--     AND entity_type = 'Track'
--     AND is_delete = false
--   ORDER BY created_at DESC
--
-- Without this index, the endpoint scans and sorts the full comments table for
-- every user-comments page request.
CREATE INDEX CONCURRENTLY IF NOT EXISTS comments_user_track_created_at_idx
    ON public.comments USING btree (user_id, created_at DESC)
    INCLUDE (comment_id)
    WHERE entity_type = 'Track'
      AND is_delete = false;
