-- Production stats show notification rows are overwhelmingly single-recipient:
-- user_ids usually contains exactly one recipient. Querying those common rows
-- through the broad GIN array index is expensive for high-fanout users, and the
-- planner can fall back to full table scans. Give the API a btree path keyed by
-- recipient and recency.
--
-- NOTE: intentionally NOT wrapped in BEGIN/COMMIT so CREATE INDEX
-- CONCURRENTLY can run without holding an ACCESS EXCLUSIVE lock on notification.
-- IF NOT EXISTS makes the migration idempotent.

CREATE INDEX CONCURRENTLY IF NOT EXISTS notification_single_recipient_user_timestamp_idx
    ON public.notification ((user_ids[1]), "timestamp" DESC, group_id DESC, type)
    WHERE array_length(user_ids, 1) = 1;

COMMENT ON INDEX public.notification_single_recipient_user_timestamp_idx IS
    'Covers notification reads for the common single-recipient user_ids array path.';
