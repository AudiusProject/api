-- handle_comment_thread
--
-- Emits a `comment_thread` notification to the parent comment's author
-- when someone replies to their comment. Mirrors apps'
-- src/tasks/entity_manager/entities/comment.py thread notification
-- block (notification type "comment_thread" with group_id
-- "comment_thread:<parent_comment_id>", specifier = reply comment_id).
--
-- Fires on comment_threads INSERT, which the indexer writes for every
-- reply (etl/processors/entity_manager/comment_create.go) after the
-- comments row exists in the same transaction.
--
-- Skips (mirror apps):
--   - parent author == reply author (self-reply)
--   - parent author muted notifications on the parent comment
--     (comment_notification_settings)
--   - parent author muted the reply author (muted_users)
--
-- Deferred (intentional): apps also drops the notification when the
-- reply author is karma-muted. See handle_comment_notification.sql
-- header for rationale.
create or replace function handle_comment_thread() returns trigger as $$
declare
  reply_row       record;
  parent_row      record;
  entity_user_id  int;
  data_entity_ref int;
  parent_mute     boolean;
begin
  -- The reply.
  select user_id, blocknumber, created_at, is_delete, is_visible
    into reply_row
    from comments
   where comment_id = new.comment_id
   limit 1;
  if not found or reply_row.is_delete or not reply_row.is_visible then
    return null;
  end if;

  -- The parent — used for both recipient and the entity context the
  -- notification payload includes.
  select user_id, entity_type, entity_id
    into parent_row
    from comments
   where comment_id = new.parent_comment_id
   limit 1;
  if not found then
    return null;
  end if;

  -- Self-reply is a no-op.
  if reply_row.user_id = parent_row.user_id then
    return null;
  end if;

  -- Resolve the entity owner for the notification payload (matches the
  -- entity-type switch in apps' comment.py).
  if parent_row.entity_type = 'Track' then
    select t.owner_id into entity_user_id
      from tracks t
     where t.track_id = parent_row.entity_id
       and t.is_current = true
     limit 1;
    data_entity_ref := parent_row.entity_id;
  elsif parent_row.entity_type = 'Event' then
    select e.user_id into entity_user_id
      from events e
     where e.event_id = parent_row.entity_id
       and e.is_deleted = false
     limit 1;
    data_entity_ref := parent_row.entity_id;
  elsif parent_row.entity_type = 'FanClub' then
    entity_user_id  := parent_row.entity_id;
    data_entity_ref := parent_row.entity_id;
  else
    -- Unknown entity_type — emit without owner context rather than skip.
    entity_user_id  := null;
    data_entity_ref := parent_row.entity_id;
  end if;

  -- Parent author muted this thread or this user.
  select exists (
    select 1 from comment_notification_settings cns
     where cns.user_id     = parent_row.user_id
       and cns.entity_type = 'Comment'
       and cns.entity_id   = new.parent_comment_id
       and cns.is_muted    = true
  ) or exists (
    select 1 from muted_users mu
     where mu.user_id       = parent_row.user_id
       and mu.muted_user_id = reply_row.user_id
       and mu.is_delete     = false
  ) into parent_mute;
  if parent_mute then
    return null;
  end if;

  insert into notification
    (blocknumber, user_ids, timestamp, type, specifier, group_id, data)
  values
    (
      reply_row.blocknumber,
      ARRAY[parent_row.user_id],
      reply_row.created_at,
      'comment_thread',
      new.comment_id::text,
      'comment_thread:' || new.parent_comment_id,
      jsonb_build_object(
        'type',            parent_row.entity_type,
        'entity_id',       data_entity_ref,
        'entity_user_id',  entity_user_id,
        'comment_user_id', reply_row.user_id,
        'comment_id',      new.comment_id
      )
    )
  on conflict do nothing;

  return null;

exception
  when others then
    raise warning 'An error occurred in %: %', tg_name, sqlerrm;
    return null;
end;
$$ language plpgsql;


do $$ begin
  create trigger on_comment_thread
    after insert on comment_threads
    for each row execute procedure handle_comment_thread();
exception
  when others then null;
end $$;
