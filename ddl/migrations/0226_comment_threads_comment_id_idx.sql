-- Supports deferred comment triggers that classify a freshly inserted comment
-- by checking whether it has a comment_threads row:
--
--   WHERE comment_id = ?
--
-- The primary key is ordered as (parent_comment_id, comment_id), which is ideal
-- for parent-thread lookups but not for this reverse lookup used during comment
-- create handling.
CREATE INDEX CONCURRENTLY IF NOT EXISTS comment_threads_comment_id_idx
    ON public.comment_threads USING btree (comment_id);
