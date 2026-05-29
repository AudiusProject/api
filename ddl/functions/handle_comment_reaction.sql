-- handle_comment_reaction
--
-- Emits a `comment_reaction` notification to the comment's author when
-- someone reacts to their comment. Mirrors apps'
-- src/tasks/entity_manager/entities/comment.py react_comment block
-- (notification type "comment_reaction" with group_id
-- "comment_reaction:<comment_id>", specifier = reacter user_id).
--
-- Fires on comment_reactions INSERT, which the indexer writes via
-- etl/processors/entity_manager/comment_react.go.
--
-- Note: NOT to be confused with handle_reaction.sql, which fires on the
-- `reactions` table for TIP reactions only. Comment reactions live in a
-- separate `comment_reactions` table with a different shape.
--
-- Skips (mirror apps):
--   - reacter == comment author (self-react)
--   - comment author muted notifications on this comment
--     (comment_notification_settings) OR muted the reacter (muted_users)
--   - if comment author IS the entity owner AND owner has notifications
--     off for the entity, skip — matches apps' track_owner_mention_mute
--
-- Deferred (intentional): karma mute. See handle_comment_notification.sql.
create or replace function handle_comment_reaction() returns trigger as $$
declare
  c_row            record;
  entity_user_id   int;
  data_entity_ref  int;
  comment_owner_mute boolean;
  owner_mute_extra   boolean;
begin
  if new.is_delete then
    return null;
  end if;

  -- The comment being reacted to. Use the stored entity_type from the
  -- comments row (apps notes clients have sometimes shipped wrong values
  -- in the reaction's metadata).
  select user_id, entity_type, entity_id, is_delete, is_visible
    into c_row
    from comments
   where comment_id = new.comment_id
   limit 1;
  if not found or c_row.is_delete or not c_row.is_visible then
    return null;
  end if;

  -- Self-react is a no-op.
  if new.user_id = c_row.user_id then
    return null;
  end if;

  -- Resolve entity context for the notification payload.
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
    entity_user_id  := null;
    data_entity_ref := c_row.entity_id;
  end if;

  -- Comment author muted notifications on this comment OR this reacter.
  select exists (
    select 1 from comment_notification_settings cns
     where cns.user_id     = c_row.user_id
       and cns.entity_type = 'Comment'
       and cns.entity_id   = new.comment_id
       and cns.is_muted    = true
  ) or exists (
    select 1 from muted_users mu
     where mu.user_id       = c_row.user_id
       and mu.muted_user_id = new.user_id
       and mu.is_delete     = false
  ) into comment_owner_mute;
  if comment_owner_mute then
    return null;
  end if;

  -- Apps' track_owner_mention_mute: if commenter is the entity owner
  -- AND owner has notifications off on the entity, drop the reaction
  -- notification too (their muted state shouldn't be circumvented by
  -- a reaction notification).
  if entity_user_id is not null and c_row.user_id = entity_user_id then
    select exists (
      select 1 from comment_notification_settings cns
       where cns.user_id     = entity_user_id
         and cns.entity_type = c_row.entity_type
         and cns.entity_id   = data_entity_ref
         and cns.is_muted    = true
    ) or exists (
      select 1 from muted_users mu
       where mu.user_id       = entity_user_id
         and mu.muted_user_id = new.user_id
         and mu.is_delete     = false
    ) into owner_mute_extra;
    if owner_mute_extra then
      return null;
    end if;
  end if;

  insert into notification
    (blocknumber, user_ids, timestamp, type, specifier, group_id, data)
  values
    (
      new.blocknumber,
      ARRAY[c_row.user_id],
      new.created_at,
      'comment_reaction',
      new.user_id::text,
      'comment_reaction:' || new.comment_id,
      jsonb_build_object(
        'type',            c_row.entity_type,
        'entity_id',       data_entity_ref,
        'entity_user_id',  entity_user_id,
        'comment_id',      new.comment_id,
        'reacter_user_id', new.user_id
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
  create trigger on_comment_reaction
    after insert on comment_reactions
    for each row execute procedure handle_comment_reaction();
exception
  when others then null;
end $$;
