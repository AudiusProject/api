-- handle_comment_notification
--
-- Emits a `comment` notification to the entity owner (track owner / event
-- host / fan-club artist) when someone leaves a top-level comment on
-- their entity.
--
-- Sibling of:
--   handle_comment.sql                          (aggregate_track counts only)
--   handle_comment_remix_contest_update.sql     (Event subscriber fan-out)
--   handle_fan_club_text_post.sql               (FanClub follower fan-out)
--
-- Mirrors apps' src/tasks/entity_manager/entities/comment.py top-level
-- `comment` Notification block (notification type "comment" with group_id
-- "comment:<entity_id>:type:<entity_type>").
--
-- Why DEFERRABLE INITIALLY DEFERRED:
--   "Top-level" means no comment_threads row for this comment_id, and
--   "owner is mentioned" means a comment_mentions row exists with the
--   owner's user_id. Both of those sibling rows are inserted AFTER the
--   comments row in the same indexer transaction. A non-deferred trigger
--   would misclassify replies as top-level and miss owner-mention skips.
--   Same pattern as handle_comment_remix_contest_update.sql.
--
-- Deferred features (intentional): apps also checks a karma-based mute
-- where a commenter's muters' aggregate follower_count must be < a
-- threshold (default 1.7M prod, 4k dev). Not ported here — keeps the
-- trigger localized and the threshold lives in apps' config not the DB.
-- If noise becomes a problem we can fold it into a follow-up.
create or replace function handle_comment_notification() returns trigger as $$
declare
  entity_user_id   int;
  data_entity_ref  int;
  group_id_str     text;
  is_reply         boolean;
  owner_mentioned  boolean;
  owner_mute       boolean;
begin
  if new.is_delete or not new.is_visible then
    return null;
  end if;

  -- Resolve recipient (entity_user_id) + data.entity_id by entity_type.
  if new.entity_type = 'Track' then
    select t.owner_id into entity_user_id
      from tracks t
     where t.track_id = new.entity_id
       and t.is_current = true
     limit 1;
    data_entity_ref := new.entity_id;
  elsif new.entity_type = 'Event' then
    select e.user_id into entity_user_id
      from events e
     where e.event_id = new.entity_id
       and e.is_deleted = false
     limit 1;
    data_entity_ref := new.entity_id;
  elsif new.entity_type = 'FanClub' then
    -- For FanClub, entity_id IS the artist's user_id.
    entity_user_id  := new.entity_id;
    data_entity_ref := new.entity_id;
  else
    return null;
  end if;

  if entity_user_id is null then
    return null;
  end if;

  -- Skip self-comment.
  if new.user_id = entity_user_id then
    return null;
  end if;

  -- Skip replies (they emit comment_thread instead, to the parent
  -- comment author). Deferred so comment_threads is visible.
  select exists (
    select 1 from comment_threads where comment_id = new.comment_id
  ) into is_reply;
  if is_reply then
    return null;
  end if;

  -- Skip if owner is mentioned in this comment (they get comment_mention
  -- instead, also more specific). Deferred so comment_mentions is visible.
  select exists (
    select 1 from comment_mentions
     where comment_id = new.comment_id
       and user_id    = entity_user_id
       and is_delete  = false
  ) into owner_mentioned;
  if owner_mentioned then
    return null;
  end if;

  -- Skip if owner muted notifications on this entity (CommentNotificationSetting
  -- with is_muted=true) OR muted this commenter (MutedUser).
  select exists (
    select 1 from comment_notification_settings cns
     where cns.user_id     = entity_user_id
       and cns.entity_type = new.entity_type
       and cns.entity_id   = data_entity_ref
       and cns.is_muted    = true
  ) or exists (
    select 1 from muted_users mu
     where mu.user_id       = entity_user_id
       and mu.muted_user_id = new.user_id
       and mu.is_delete     = false
  ) into owner_mute;
  if owner_mute then
    return null;
  end if;

  group_id_str := 'comment:' || data_entity_ref || ':type:' || new.entity_type;

  insert into notification
    (blocknumber, user_ids, timestamp, type, specifier, group_id, data)
  values
    (
      new.blocknumber,
      ARRAY[entity_user_id],
      new.created_at,
      'comment',
      new.comment_id::text,
      group_id_str,
      jsonb_build_object(
        'type',            new.entity_type,
        'entity_id',       data_entity_ref,
        'comment_user_id', new.user_id,
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
  create constraint trigger on_comment_notification
    after insert on comments
    deferrable initially deferred
    for each row execute procedure handle_comment_notification();
exception
  when others then null;
end $$;
