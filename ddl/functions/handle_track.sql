create or replace function track_is_public(track record) returns boolean as $$
begin
  return track.is_unlisted = false
     and track.is_available = true
     and track.is_delete = false
     and track.stem_of is null
     and track.access_authorities is null;
end
$$ LANGUAGE plpgsql;

create or replace function track_should_notify(old_track tracks, new_track record, tg_op varchar) returns boolean as $$
begin
  if tg_op = 'UPDATE' and old_track.track_id is not null then
    return not track_is_public(old_track) and track_is_public(new_track);
  else
    return tg_op = 'INSERT'
      and track_is_public(new_track)
    ;
  end if;
end
$$ LANGUAGE plpgsql;

create or replace function handle_track() returns trigger as $$
declare
  parent_track_owner_id int;
  subscriber_user_ids int[];
begin
  insert into aggregate_track (track_id) values (new.track_id) on conflict do nothing;
  insert into aggregate_user (user_id) values (new.owner_id) on conflict do nothing;

  update aggregate_user
  set (track_count, total_track_count) = (
    select
      count(*) filter (where t.is_unlisted = false),
      count(*)
    from tracks t
    where t.is_current is true
      and t.is_delete is false
      and t.is_available is true
      and t.stem_of is null
      and t.access_authorities is null
      and t.owner_id = new.owner_id
  )
  where user_id = new.owner_id
  ;

  -- If new track or newly unlisted track, create notification
  begin
    if track_should_notify(OLD, new, TG_OP) AND new.is_playlist_upload = FALSE THEN
      select array(
        select subscriber_id
          from subscriptions
          where is_current and
          not is_delete and
          user_id=new.owner_id
      ) into subscriber_user_ids;

      if array_length(subscriber_user_ids, 1)	> 0 then
        insert into notification
          (blocknumber, user_ids, timestamp, type, specifier, group_id, data)
        values
        (
          new.blocknumber,
          subscriber_user_ids,
          new.updated_at,
          'create',
          new.track_id,
          'create:track:user_id:' || new.owner_id,
          json_build_object('track_id', new.track_id)
        )
        on conflict do nothing;
      end if;
    end if;
	exception
    when others then
      raise warning 'An error occurred in %: %', tg_name, sqlerrm;
  end;

  -- If new remix or newly unlisted remix, create notification
  begin
    if track_should_notify(OLD, new, TG_OP) AND new.remix_of is not null THEN
      select owner_id into parent_track_owner_id from tracks where is_current and track_id = (new.remix_of->'tracks'->0->>'parent_track_id')::int limit 1;
      if parent_track_owner_id is not null then
        insert into notification
        (blocknumber, user_ids, timestamp, type, specifier, group_id, data)
        values
        (
          new.blocknumber,
          ARRAY [parent_track_owner_id],
          new.updated_at,
          'remix',
          new.owner_id,
          'remix:track:' || new.track_id || ':parent_track:' || (new.remix_of->'tracks'->0->>'parent_track_id')::int,
          json_build_object('track_id', new.track_id, 'parent_track_id', (new.remix_of->'tracks'->0->>'parent_track_id')::int)
        )
        on conflict do nothing;
      end if;
    end if;
	exception
    when others then
      raise warning 'An error occurred in %: %', tg_name, sqlerrm;
  end;

  -- If new remix is a submission to an active remix contest, check for milestone notifications
  begin
    if track_should_notify(OLD, new, TG_OP) AND new.remix_of is not null THEN
      declare
        contest_event_id int;
        contest_creator_id int;
        submission_count int;
        milestone int;
        parent_track_id int := (new.remix_of->'tracks'->0->>'parent_track_id')::int;
      begin
        select event_id, user_id
        into contest_event_id, contest_creator_id
        from events
        where event_type = 'remix_contest'
          and is_deleted = false
          and end_date > now()
          and entity_id = parent_track_id
        limit 1;

        if contest_event_id is not null then
          -- Count submissions for this contest (only those after contest start)
          select count(*) into submission_count
          from tracks t
          join events e on e.event_type = 'remix_contest'
            and e.is_deleted = false
            and e.entity_id = parent_track_id
          where t.is_current = true
            and t.is_delete = false
            and t.remix_of is not null
            and (t.remix_of->'tracks'->0->>'parent_track_id')::int = parent_track_id
            and t.created_at >= e.created_at;

          -- For each milestone, insert notification if this is the Nth submission
          FOREACH milestone IN ARRAY ARRAY[1, 10, 50] LOOP
            IF submission_count = milestone THEN
              insert into notification
                (blocknumber, user_ids, timestamp, type, specifier, group_id, data)
              values
                (
                  new.blocknumber,
                  ARRAY [contest_creator_id],
                  new.updated_at,
                  'artist_remix_contest_submissions',
                  milestone || ':' || contest_event_id,
                  'artist_remix_contest_submissions:' || contest_event_id || ':' || milestone,
                  json_build_object(
                    'event_id', contest_event_id,
                    'milestone', milestone,
                    'entity_id', parent_track_id
                  )
                )
              on conflict do nothing;
            END IF;
          END LOOP;
        end if;
      end;
    end if;
    exception
      when others then
        raise warning 'An error occurred in %: %', tg_name, sqlerrm;
  end;

  -- If a track with an active remix contest transitions from unlisted to public,
  -- create fan_remix_contest_started notifications for the contest creator's
  -- followers and the track's savers. Mirrors handle_event.sql for the case
  -- where the contest was created while the track was still unlisted.
  begin
    if TG_OP = 'UPDATE' and OLD.is_unlisted = true and new.is_unlisted = false THEN
      declare
        contest_event_id int;
        contest_creator_id int;
        notified_user_id int;
      begin
        select event_id, user_id
        into contest_event_id, contest_creator_id
        from events
        where event_type = 'remix_contest'
          and is_deleted = false
          and end_date > now()
          and entity_id = new.track_id
        limit 1;

        if contest_event_id is not null then
          for notified_user_id in
            select distinct user_id
            from (
              -- Get followers of the contest creator
              select f.follower_user_id as user_id
              from follows f
              where f.followee_user_id = contest_creator_id
                and f.is_current = true
                and f.is_delete = false
              union
              -- Get users who favorited the track
              select s.user_id
              from saves s
              where s.save_item_id = new.track_id
                and s.save_type = 'track'
                and s.is_current = true
                and s.is_delete = false
            ) as users_to_notify
          loop
            insert into notification
              (blocknumber, user_ids, timestamp, type, specifier, group_id, data)
            values
              (
                new.blocknumber,
                ARRAY [notified_user_id],
                new.updated_at,
                'fan_remix_contest_started',
                notified_user_id,
                'fan_remix_contest_started:' || new.track_id || ':user:' || contest_creator_id,
                json_build_object(
                  'entity_user_id', new.owner_id,
                  'entity_id', new.track_id
                )
              )
            on conflict do nothing;
          end loop;
        end if;
      end;
    end if;
    exception
      when others then
        raise warning 'An error occurred in %: %', tg_name, sqlerrm;
  end;

  return null;

exception
    when others then
      raise warning 'An error occurred in %: %', tg_name, sqlerrm;
      raise;

end;
$$ language plpgsql;



do $$ begin
  create trigger on_track
  after insert or update on tracks
  for each row execute procedure handle_track();
exception
  when others then null;
end $$;
