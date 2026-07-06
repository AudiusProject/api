-- The notification API handles the overwhelmingly common single-recipient path
-- with notification_single_recipient_user_timestamp_idx. Keep the fallback
-- multi-recipient overlap scan off the all-row GIN index so high-notification
-- users do not recheck large numbers of single-recipient rows.
--
-- NOTE: intentionally NOT wrapped in BEGIN/COMMIT so CREATE INDEX
-- CONCURRENTLY can run without holding an ACCESS EXCLUSIVE lock on notification.

CREATE INDEX CONCURRENTLY IF NOT EXISTS notification_multi_recipient_user_ids_idx
    ON public.notification USING gin (user_ids)
    WHERE COALESCE(array_length(user_ids, 1), 0) != 1;

COMMENT ON INDEX public.notification_multi_recipient_user_ids_idx IS
    'Covers notification reads for the uncommon multi-recipient user_ids array path.';
