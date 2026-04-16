-- Generalise the subscriptions table to support following entities other
-- than users. Existing rows always represent User->User subscriptions; the
-- new entity_type column defaults to 'User' so they're unchanged. New rows
-- (e.g. "user follows a remix-contest event") set entity_type='Event' and
-- mirror the target's id into entity_id while keeping the legacy user_id
-- column populated so the existing (subscriber_id, user_id) uniqueness
-- constraint remains collision-free across subscription kinds.
--
-- NOTE: intentionally NOT wrapped in BEGIN/COMMIT so that
-- CREATE INDEX CONCURRENTLY can run without holding a long
-- ACCESS EXCLUSIVE lock on subscriptions. The previous attempt
-- (0195) wrapped everything in a transaction and built the index
-- non-concurrently, which locked the table for the duration of
-- the build and caused an API outage. All statements are guarded
-- with IF NOT EXISTS so this migration is a no-op on environments
-- where 0195 already committed.

alter table subscriptions
    add column if not exists entity_type text not null default 'User';

alter table subscriptions
    add column if not exists entity_id integer;

create index concurrently if not exists subscriptions_entity_type_entity_id_idx
    on subscriptions (entity_type, entity_id)
    where is_current = true and is_delete = false;
