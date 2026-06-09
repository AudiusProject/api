-- chat_chat_id_idx duplicates chat_pkey: both are btree indexes on chat_id.
-- Keep the unique primary-key index and remove the redundant non-unique copy.

drop index concurrently if exists public.chat_chat_id_idx;
