-- handle_tastemaker
--
-- Emits a `tastemaker` notification when the tastemaker challenge
-- processor (challenge_id 't') mints a user_challenges row. Each row
-- corresponds to one user who reposted or saved a track that later went
-- trending. The notification tells the tastemaker user that they were
-- early to a now-trending track.
--
-- Sibling of handle_user_challenges.sql which already emits the generic
-- `challenge_reward` notification for all challenge completions. This
-- trigger is the type-specific layer that matches apps' tastemaker
-- notification (src/tasks/index_tastemaker.py).
--
-- Specifier shape from jobs/challenges/tastemaker.go is
-- "<hex_user_id>:t:<hex_track_id>" — we parse the trailing hex track_id,
-- look up its owner from `tracks`, and infer the action (repost takes
-- precedence over save, matching apps' dedupe_notifications_by_group_id).
create or replace function handle_tastemaker() returns trigger as $$
declare
  track_hex     text;
  track_id_int  bigint;
  owner_id_int  int;
  action_str    text;
begin
  -- WHEN clause on the trigger gates challenge_id='t', but defend in
  -- depth here too in case the trigger is invoked another way.
  if new.challenge_id <> 't' then
    return null;
  end if;

  -- Parse trailing hex segment "<user_hex>:t:<track_hex>" → track_id.
  track_hex := split_part(new.specifier, ':', 3);
  if track_hex !~ '^[0-9a-f]+$' then
    return null;
  end if;
  track_id_int := ('x' || lpad(track_hex, 16, '0'))::bit(64)::bigint;
  if track_id_int <= 0 then
    return null;
  end if;

  select t.owner_id
    into owner_id_int
    from tracks t
   where t.track_id = track_id_int
     and t.is_current = true
   limit 1;
  if owner_id_int is null then
    return null;
  end if;

  -- Repost takes precedence over save when a user is in both lists for
  -- the same track — matches apps' dedupe_notifications_by_group_id
  -- where repost_notifications win over save_notifications.
  if exists (
    select 1
      from reposts
     where user_id = new.user_id
       and repost_item_id = track_id_int
       and repost_type = 'track'
       and is_current = true
       and is_delete = false
  ) then
    action_str := 'repost';
  else
    action_str := 'save';
  end if;

  insert into notification
    (blocknumber, user_ids, timestamp, type, specifier, group_id, data)
  values
    (
      new.completed_blocknumber,
      ARRAY[new.user_id],
      new.completed_at,
      'tastemaker',
      track_id_int::text,
      'tastemaker_user_id:' || new.user_id || ':tastemaker_item_id:' || track_id_int,
      jsonb_build_object(
        'tastemaker_item_id',       track_id_int,
        'tastemaker_item_type',     'track',
        'tastemaker_item_owner_id', owner_id_int,
        'action',                   action_str,
        'tastemaker_user_id',       new.user_id
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
  -- Fire only on INSERT (not UPDATE) so the notification is minted
  -- exactly once per (user_id, track_id) pair — UpsertUserChallenge
  -- hits its ON CONFLICT DO UPDATE branch on re-runs, which does not
  -- fire AFTER INSERT triggers.
  create trigger on_tastemaker_user_challenge
    after insert on user_challenges
    for each row when (new.challenge_id = 't')
    execute procedure handle_tastemaker();
exception
  when others then null;
end $$;
