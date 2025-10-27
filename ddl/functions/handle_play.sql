create or replace function handle_play() returns trigger as $$
declare
    new_listen_count int;
    milestone int;
    owner_user_id int;
begin

    -- update aggregate_plays
    insert into aggregate_plays (play_item_id, count) values (new.play_item_id, 0) on conflict do nothing;

    update aggregate_plays
        set count = count + 1
        where play_item_id = new.play_item_id
        returning count into new_listen_count;

    -- update aggregate_monthly_plays
    insert into aggregate_monthly_plays (play_item_id, timestamp, country, count)
    values (new.play_item_id, date_trunc('month', new.created_at), coalesce(new.country, ''), 0)
    on conflict do nothing;

    update aggregate_monthly_plays
        set count = count + 1
        where play_item_id = new.play_item_id
        and timestamp = date_trunc('month', new.created_at)
        and country = coalesce(new.country, '');

    select new_listen_count
        into milestone
        where new_listen_count in (10,25,50,100,250,500,1000,2500,5000,10000,25000,50000,100000,250000,500000,1000000);

    if milestone is not null then
        insert into milestones
            (id, name, threshold, slot, timestamp)
        values
            (new.play_item_id, 'LISTEN_COUNT', milestone, new.slot, new.created_at)
        on conflict do nothing;
        select tracks.owner_id into owner_user_id from tracks where is_current and track_id = new.play_item_id;
        if owner_user_id is not null then
            insert into notification
                (user_ids, specifier, group_id, type, slot, timestamp, data)
                values
                (
                    array[owner_user_id],
                    owner_user_id,
                    'milestone:LISTEN_COUNT:id:' || new.play_item_id || ':threshold:' || milestone,
                    'milestone',
                    new.slot,
                    new.created_at,
                    json_build_object('type', 'LISTEN_COUNT', 'track_id', new.play_item_id, 'threshold', milestone)
                )
            on conflict do nothing;
        end if;
    end if;

    -- update listener's aggregates if applicable
    if new.user_id is not null then
        -- Insert or update user_distinct_play_hours
        -- Only increment if the play's hour is newer than the user's last updated hour
        insert into user_distinct_play_hours (user_id, hours_with_play, updated_at)
        values (new.user_id, 1, date_trunc('hour', new.created_at))
        on conflict (user_id) do update set
            hours_with_play = case
                when date_trunc('hour', new.created_at) > date_trunc('hour', user_distinct_play_hours.updated_at)
                then user_distinct_play_hours.hours_with_play + 1
                else user_distinct_play_hours.hours_with_play
            end,
            updated_at = case
                when date_trunc('hour', new.created_at) > date_trunc('hour', user_distinct_play_hours.updated_at)
                then new.created_at
                else user_distinct_play_hours.updated_at
            end;

        -- update user_distinct_play_tracks
        -- Only increment if this is the first time this user has played this track
        insert into user_distinct_play_tracks (user_id, track_count, updated_at)
        values (new.user_id, 1, new.created_at)
        on conflict (user_id) do update set
            track_count = case
                when not exists (
                    select 1 from plays p
                    where p.user_id = new.user_id
                    and p.play_item_id = new.play_item_id
                    and p.id != new.id
                )
                then user_distinct_play_tracks.track_count + 1
                else user_distinct_play_tracks.track_count
            end,
            updated_at = new.created_at;
    end if;

    return null;

exception
  when others then
    raise warning 'An error occurred in %: %', tg_name, sqlerrm;
    raise;

end;
$$ language plpgsql;

do $$ begin
    create trigger on_play
        after insert on plays
        for each row execute procedure handle_play();
exception
  when others then null;
end $$;