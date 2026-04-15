begin;

-- Generalise the subscriptions table to support following entities other
-- than users. Existing rows always represent User→User subscriptions; the
-- new entity_type column defaults to 'User' so they're unchanged. New rows
-- (e.g. "user follows a remix-contest event") set entity_type='Event' and
-- mirror the target's id into entity_id while keeping the legacy user_id
-- column populated so the existing (subscriber_id, user_id) uniqueness
-- constraint remains collision-free across subscription kinds.

alter table subscriptions
    add column if not exists entity_type text not null default 'User';

alter table subscriptions
    add column if not exists entity_id integer;

update subscriptions
    set entity_type = 'User'
    where entity_type is null or entity_type = '';

create index if not exists subscriptions_entity_type_entity_id_idx
    on subscriptions (entity_type, entity_id)
    where is_current = true and is_delete = false;

commit;
