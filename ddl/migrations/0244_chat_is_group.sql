-- Group DMs: chats with more than two members.
--
-- is_group is set once at chat.create (more than two invites) and never
-- changes, so a group stays a group after members leave. 1:1 rules that key
-- off "an existing chat between two users" (chat_allowed, blast fan-out)
-- only look at chats where is_group is false.

BEGIN;

ALTER TABLE chat ADD COLUMN IF NOT EXISTS is_group boolean NOT NULL DEFAULT false;

COMMENT ON COLUMN chat.is_group IS 'True for group DMs (created with more than two members). Group chats never grant 1:1 messaging rights and never receive blasts.';

COMMIT;
