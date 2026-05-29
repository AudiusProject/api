-- handle_comment_mention
--
-- Emits a `comment_mention` notification to a mentioned user when a
-- comment_mentions row is inserted (or undeleted). Mirrors apps'
-- src/tasks/entity_manager/entities/comment.py mention notification
-- block (notification type "comment_mention" with group_id
-- "comment_mention:<comment_id>").
--
-- Why this fires on comment_mentions (not comments):
--   The mention rows are written AFTER the comments row in the same
--   indexer transaction (etl/processors/entity_manager/comment_create.go).
--   Hooking the child table lets a plain AFTER INSERT trigger see
--   everything it needs without DEFERRED gymnastics — the comments row
--   already exists by the time comment_mentions is inserted.
--
-- Skips (mirror apps):
--   - mention == commenter (self-mention)
--   - mentioned user has muted the commenter (muted_users)
--   - if mention == entity owner AND owner has notifications off for
--     this entity, skip — the entity owner already opted out
--
-- Deferred (intentional): apps also drops mentions when the commenter is
-- karma-muted (1.7M-follower-aggregate threshold across the muting
-- users). Not ported here for the same reason as
-- handle_comment_notification.sql — see header there.
create or replace function handle_comment_mention() returns trigger as $$
declare
  c_row            record;
  entity_user_id   int;
  data_entity_ref  int;
  is_self_mention  boolean;
  mention_muted    boolean;
  owner_mute       boolean;
  is_owner_mention boolean;
begin
  if new.is_delete then
    return null;
  end if;

  -- Fetch the parent comment for entity context + author.
  select user_id, entity_type, entity_id, blocknumber, created_at, is_delete, is_visible
    into c_row
    from comments
   where comment_id = new.comment_id
   limit 1;
  if not found or c_row.is_delete or not c_row.is_visible then
    return null;
  end if;

  -- Self-mention is a no-op.
  if new.user_id = c_row.user_id then
    return null;
  end if;

  -- Resolve entity owner — used for the "owner has notifications off"
  -- gate when the mention IS the owner.
  if c_row.entity_type = 'Track' then
    select t.owner_id into entity_user_id
      from tracks t
     where t.track_id = c_row.entity_id
       and t.is_current = true
     limit 1;
    data_entity_ref := c_row.entity_id;
  elsif c_row.entity_type = 'Event' then
    select e.user_id into entity_user_id
      from events e
     where e.event_id = c_row.entity_id
       and e.is_deleted = false
     limit 1;
    data_entity_ref := c_row.entity_id;
  elsif c_row.entity_type = 'FanClub' then
    entity_user_id  := c_row.entity_id;
    data_entity_ref := c_row.entity_id;
  else
    return null;
  end if;

  is_owner_mention := (entity_user_id is not null and new.user_id = entity_user_id);

  -- Mentioned user has muted the commenter — skip.
  select exists (
    select 1 from muted_users mu
     where mu.user_id       = new.user_id
       and mu.muted_user_id = c_row.user_id
       and mu.is_delete     = false
  ) into mention_muted;
  if mention_muted then
    return null;
  end if;

  -- If the mention is the entity owner AND the owner muted notifications
  -- on this entity, skip — matches apps' track_owner_mention_mute logic.
  if is_owner_mention then
    select exists (
      select 1 from comment_notification_settings cns
       where cns.user_id     = entity_user_id
         and cns.entity_type = c_row.entity_type
         and cns.entity_id   = data_entity_ref
         and cns.is_muted    = true
    ) or exists (
      select 1 from muted_users mu
       where mu.user_id       = entity_user_id
         and mu.muted_user_id = c_row.user_id
         and mu.is_delete     = false
    ) into owner_mute;
    if owner_mute then
      return null;
    end if;
  end if;

  insert into notification
    (blocknumber, user_ids, timestamp, type, specifier, group_id, data)
  values
    (
      c_row.blocknumber,
      ARRAY[new.user_id],
      c_row.created_at,
      'comment_mention',
      new.user_id::text,
      'comment_mention:' || new.comment_id,
      jsonb_build_object(
        'type',            c_row.entity_type,
        'entity_id',       data_entity_ref,
        'entity_user_id',  entity_user_id,
        'comment_user_id', c_row.user_id,
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
  create trigger on_comment_mention
    after insert on comment_mentions
    for each row execute procedure handle_comment_mention();
exception
  when others then null;
end $$;
