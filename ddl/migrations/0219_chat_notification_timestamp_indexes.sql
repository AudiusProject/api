-- Pedalboard's DM notification worker scans for chat messages/reactions newer
-- than its timestamp cursor, then joins those rows to chat_member. These
-- timestamp-leading indexes let Postgres start from the small "new since
-- cursor" slice instead of scanning the whole chat table every poll.
--
-- NOTE: intentionally NOT wrapped in BEGIN/COMMIT so CREATE INDEX
-- CONCURRENTLY can run without holding an ACCESS EXCLUSIVE lock on chat
-- tables. IF NOT EXISTS makes the migration idempotent.

create index concurrently if not exists chat_message_non_blast_created_at_idx
    on public.chat_message (created_at, chat_id, user_id)
    where blast_id is null;

comment on index public.chat_message_non_blast_created_at_idx is
    'Supports DM notification polling for non-blast chat messages by timestamp cursor.';

create index concurrently if not exists chat_message_reactions_updated_at_idx
    on public.chat_message_reactions (updated_at, message_id, user_id);

comment on index public.chat_message_reactions_updated_at_idx is
    'Supports DM reaction notification polling by updated_at cursor.';
