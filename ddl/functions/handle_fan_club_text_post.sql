-- handle_fan_club_text_post
--
-- Emits a `fan_club_text_post` notification to followers + artist-coin
-- holders when a fan-club artist posts a top-level "text update" on
-- their fan club. Mirrors apps'
-- src/tasks/entity_manager/entities/comment.py FanClub block.
--
-- Sibling of:
--   handle_comment.sql                          (aggregates)
--   handle_comment_notification.sql             (entity-owner notif)
--   handle_comment_remix_contest_update.sql     (Event-host fan-out)
--
-- Fan-club entity_id IS the artist's user_id (apps uses entity_id as
-- the artist's user identifier for FanClub-typed comments). The post
-- author MUST be the artist themselves — a fan's comment on the fan
-- club is just a regular comment, not a "text post".
--
-- Recipients = (followers ∪ artist-coin holders) - { artist }.
-- Per-recipient row is required: each row has a single user_id in the
-- group_id (matches apps' "fan_club_text_post:<comment_id>:user:<artist>"
-- group_id with specifier=recipient_id, so the unique constraint
-- (group_id, specifier) dedupes correctly across recipients).
--
-- Why DEFERRABLE INITIALLY DEFERRED: "top-level" = no comment_threads
-- row, which is inserted later in the same indexer transaction. Same
-- pattern as handle_comment_remix_contest_update.sql.
create or replace function handle_fan_club_text_post() returns trigger as $$
declare
  artist_user_id  int;
  recipient_id    int;
  group_id_str    text;
  data_jsonb      jsonb;
  is_reply        boolean;
begin
  if new.entity_type <> 'FanClub' or new.is_delete or not new.is_visible then
    return null;
  end if;

  -- Artist = new.entity_id (the fan club's owner). Post author must be
  -- the artist; fan comments don't fan out.
  artist_user_id := new.entity_id;
  if new.user_id <> artist_user_id then
    return null;
  end if;

  -- Skip replies — only root-level posts fan out.
  select exists (
    select 1 from comment_threads where comment_id = new.comment_id
  ) into is_reply;
  if is_reply then
    return null;
  end if;

  group_id_str := 'fan_club_text_post:' || new.comment_id
                  || ':user:' || artist_user_id;
  data_jsonb   := jsonb_build_object(
    'entity_user_id', artist_user_id,
    'comment_id',     new.comment_id
  );

  -- Fan out: followers ∪ coin holders, excluding the artist.
  for recipient_id in
    select u
      from (
        select follower_user_id as u
          from follows
         where followee_user_id = artist_user_id
           and is_current = true
           and is_delete  = false
        union
        select sub.user_id as u
          from sol_user_balances sub
          join artist_coins ac on ac.mint = sub.mint
         where ac.user_id   = artist_user_id
           and sub.balance > 0
      ) recipients
     where u <> artist_user_id
  loop
    insert into notification
      (blocknumber, user_ids, timestamp, type, specifier, group_id, data)
    values
      (
        new.blocknumber,
        ARRAY[recipient_id],
        new.created_at,
        'fan_club_text_post',
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
  create constraint trigger on_fan_club_text_post
    after insert on comments
    deferrable initially deferred
    for each row execute procedure handle_fan_club_text_post();
exception
  when others then null;
end $$;
