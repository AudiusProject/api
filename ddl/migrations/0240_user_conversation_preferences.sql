-- Per-user inbox category for direct-message chats.
--
-- A user can file each of their chats into a "priority" or "general" inbox
-- (RPC chat.set_category). The choice is private to that user: the other
-- member of the same chat keeps their own row (or none). A chat with no row
-- here is "uncategorized", so clearing the category deletes the row rather
-- than storing a null. Blast pseudo-chats are never categorized.
--
-- updated_at carries the RPC's relayed_at timestamp so that a late-arriving
-- set_category can't overwrite a newer one (same idiom as chat_permissions).
--
-- The primary key is exactly the (user_id, chat_id) pair every reader joins
-- on, so no additional index is needed.

BEGIN;

CREATE TABLE IF NOT EXISTS user_conversation_preferences (
    user_id    INTEGER NOT NULL,
    chat_id    TEXT    NOT NULL,
    category   TEXT    NOT NULL CHECK (category IN ('priority', 'general')),
    updated_at TIMESTAMP WITHOUT TIME ZONE NOT NULL,
    PRIMARY KEY (user_id, chat_id)
);

COMMENT ON TABLE user_conversation_preferences IS
    'Per-user inbox category (priority | general) for a direct-message chat, set via the chat.set_category RPC. Absence of a row means the chat is uncategorized for that user.';

COMMIT;
