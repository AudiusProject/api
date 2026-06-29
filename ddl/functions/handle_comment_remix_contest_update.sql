-- handle_comment_remix_contest_update
--
-- Emits a `remix_contest_update` notification to every event subscriber
-- (except the host) when the contest host posts a TOP-LEVEL comment on
-- their own remix-contest event.
--
-- Sibling of handle_event.sql / handle_track.sql which already emit the
-- other three contest notifications:
--   - handle_event.sql:  fan_remix_contest_started
--   - handle_track.sql:  artist_remix_contest_submissions
--                        fan_remix_contest_submission
--
-- Why DEFERRABLE INITIALLY DEFERRED:
--   "Top-level" is determined by the absence of a comment_threads row for
--   this comment_id. The indexer inserts that row AFTER the comments row,
--   in the same transaction. A plain AFTER INSERT trigger on comments
--   would fire before comment_threads is populated and incorrectly treat
--   every reply as top-level. A deferred constraint trigger fires at
--   commit time, by which point both rows are visible.
create or replace function handle_comment_remix_contest_update() returns trigger as $$
declare
  event_host_id     int;
  contest_track_id  int;
  recipient_id      int;
  group_id_str      text;
  data_jsonb        jsonb;
begin
  -- Cheap pre-filters first.
  if new.entity_type <> 'Event' or new.is_delete or not new.is_visible then
    return null;
  end if;

  -- Bail if this comment is a reply (a comment_threads row was inserted
  -- alongside or before commit). Replies do not produce
  -- remix_contest_update — only the host's top-level posts do.
  if exists (
    select 1 from comment_threads where comment_id = new.comment_id
  ) then
    return null;
  end if;

  -- The event must exist, must be a remix_contest, and the commenter
  -- must be the host. entity_id on events is the parent track id.
  select e.user_id, e.entity_id
    into event_host_id, contest_track_id
    from events e
   where e.event_id = new.entity_id
     and e.event_type = 'remix_contest'
     and e.is_deleted = false
   limit 1;

  if event_host_id is null or event_host_id <> new.user_id then
    return null;
  end if;

  group_id_str := 'remix_contest_update:' || new.comment_id || ':event:' || new.entity_id;
  data_jsonb   := jsonb_build_object(
    'event_id',       new.entity_id,
    'entity_id',      contest_track_id,
    'entity_user_id', event_host_id,
    'comment_id',     new.comment_id
  );

  -- Fan out to subscribers, excluding the host (they have their own
  -- view of the post).
  for recipient_id in
    select s.subscriber_id
      from subscriptions s
     where s.entity_type = 'Event'
       and s.entity_id = new.entity_id
       and s.is_current = true
       and s.is_delete = false
       and s.subscriber_id <> event_host_id
  loop
    insert into notification
      (blocknumber, user_ids, timestamp, type, specifier, group_id, data)
    values
      (
        new.blocknumber,
        ARRAY[recipient_id],
        new.created_at,
        'remix_contest_update',
        recipient_id::text,
        group_id_str,
        data_jsonb
      )
    on conflict do nothing;
  end loop;

  return null;

exception
  when others then
    raise warning 'An error occurred in %: %', tg_name, sqlerrm;
    return null;
end;
$$ language plpgsql;


do $$ begin
  -- Deferred so it fires at commit time, after the sibling
  -- comment_threads insert (if any) is also visible. Without that, we'd
  -- misclassify every reply as a top-level post.
  create constraint trigger on_comment_remix_contest_update
    after insert on comments
    deferrable initially deferred
    for each row execute procedure handle_comment_remix_contest_update();
exception
  when others then null;
end $$;
