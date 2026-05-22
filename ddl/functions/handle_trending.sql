-- handle_trending
--
-- Emits a `trending` or `trending_underground` notification when the
-- trending challenge processor mints a user_challenges row for
-- challenge_id 'tt' / 'tut'. These are the "your track is trending"
-- notifications shown to the track owner.
--
-- (Trending playlists — challenge_id 'tp' — were a product feature that
-- has since been removed, so they're intentionally not handled here.
-- handle_user_challenges.sql still excludes 'tp' from the
-- claimable_reward path on line 14 for historical rows.)
--
-- Sibling of handle_user_challenges.sql which already emits the generic
-- `challenge_reward` notification for all challenge completions. This
-- trigger is the type-specific layer that matches apps'
-- index_trending.py notifications.
--
-- Specifier shape from jobs/challenges/trending.go is "<week>:<rank>"
-- (e.g. "2026-05-22:3"). Entity id is recovered from `trending_results`,
-- which the same processor wrote earlier in this transaction.
create or replace function handle_trending() returns trigger as $$
declare
  rank_int        int;
  week_date       date;
  entity_id_str   text;
  entity_id_int   bigint;
  notif_type      text;
  trend_type      text;
  ts_epoch        bigint;
  data_jsonb      jsonb;
begin
  if new.challenge_id not in ('tt', 'tut') then
    return null;
  end if;

  case new.challenge_id
    when 'tt'  then notif_type := 'trending';             trend_type := 'TRACKS';
    when 'tut' then notif_type := 'trending_underground'; trend_type := 'UNDERGROUND_TRACKS';
  end case;

  -- Specifier: "<YYYY-MM-DD>:<rank>"
  begin
    week_date := split_part(new.specifier, ':', 1)::date;
    rank_int  := split_part(new.specifier, ':', 2)::int;
  exception when others then
    return null;
  end;

  -- Recover entity id from the trending_results row the processor wrote
  -- earlier in this transaction. PK is (rank, type, version, week); we
  -- pin to NEW.user_id so we ignore any unrelated version rows.
  select id
    into entity_id_str
    from trending_results
   where rank    = rank_int
     and type    = trend_type
     and week    = week_date
     and user_id = new.user_id
   limit 1;
  if entity_id_str is null then
    return null;
  end if;
  begin
    entity_id_int := entity_id_str::bigint;
  exception when others then
    return null;
  end;

  -- timestamp suffix matches apps: epoch seconds of the recompute. We
  -- use completed_at which is set by UpsertUserChallenge to now() on
  -- the first insert — close enough to the recompute moment.
  ts_epoch := extract(epoch from new.completed_at)::bigint;

  data_jsonb := jsonb_build_object(
    'time_range', 'week',
    'genre',      'all',
    'rank',       rank_int,
    'track_id',   entity_id_int
  );

  insert into notification
    (blocknumber, user_ids, timestamp, type, specifier, group_id, data)
  values
    (
      new.completed_blocknumber,
      ARRAY[new.user_id],
      new.completed_at,
      notif_type,
      entity_id_int::text,
      notif_type
        || ':time_range:week:genre:all:rank:' || rank_int
        || ':track_id:' || entity_id_int
        || ':timestamp:' || ts_epoch,
      data_jsonb
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
  -- exactly once per (challenge_id, week, rank) — re-runs hit
  -- UpsertUserChallenge's ON CONFLICT DO UPDATE branch and do not
  -- fire AFTER INSERT triggers.
  create trigger on_trending_user_challenge
    after insert on user_challenges
    for each row when (new.challenge_id in ('tt', 'tut'))
    execute procedure handle_trending();
exception
  when others then null;
end $$;
