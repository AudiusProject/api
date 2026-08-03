-- Chat unread/message fanout queries constrain by chat_id and created_at.
-- The old partial index led with created_at and had 0 scans in production,
-- while chat_message lookups were producing heavy temp I/O. Lead with chat_id
-- so the per-chat range checks can use a small ordered slice.

CREATE INDEX CONCURRENTLY IF NOT EXISTS chat_message_chat_created_non_blast_idx
    ON public.chat_message USING btree (chat_id, created_at, user_id)
    WHERE blast_id IS NULL;

DROP INDEX CONCURRENTLY IF EXISTS public.chat_message_non_blast_created_at_idx;
