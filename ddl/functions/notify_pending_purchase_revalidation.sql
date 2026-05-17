-- Notifies Go to re-run validatePurchase for sol_purchases rows that became
-- eligible because a tracks/playlists row's blocknumber advanced past their
-- valid_after_blocknumber. The trigger does no validation itself — it just
-- signals which content_id changed; the Go indexer LISTENs on
-- 'pending_purchase_revalidation' and re-runs the existing validator so the
-- math stays in one place. A periodic sweep in Go catches anything missed
-- (NOTIFY is dropped if no listener is connected at the moment).
create or replace function notify_pending_purchase_revalidation() returns trigger as $$
declare
  v_content_type text;
  v_content_id int;
  v_blocknumber int;
begin
  if tg_table_name = 'tracks' then
    v_content_type := 'track';
    v_content_id := new.track_id;
  elsif tg_table_name = 'playlists' then
    v_content_type := 'album';
    v_content_id := new.playlist_id;
  else
    return null;
  end if;

  v_blocknumber := new.blocknumber;
  if v_blocknumber is null then
    return null;
  end if;

  -- Cheap EXISTS guard: tracks/playlists update churn dwarfs the pending
  -- purchase set, so it's almost always a no-op. Uses sol_purchases_valid_idx
  -- + sol_purchases_content_idx.
  if exists (
    select 1 from sol_purchases sp
    where sp.content_id = v_content_id
      and sp.content_type = v_content_type
      and sp.is_valid is null
      and sp.valid_after_blocknumber <= v_blocknumber
  ) then
    perform pg_notify(
      'pending_purchase_revalidation',
      v_content_type || ':' || v_content_id::text
    );
  end if;

  return null;
exception
  when others then
    -- Never let a notify failure break a tracks/playlists write. The sweep
    -- will catch any pending rows that don't get notified.
    raise warning 'An error occurred in %: %', tg_name, sqlerrm;
    return null;
end;
$$ language plpgsql;


do $$ begin
  create trigger on_track_notify_pending_purchase_revalidation
  after insert or update of blocknumber on tracks
  for each row execute procedure notify_pending_purchase_revalidation();
exception
  when others then null;
end $$;

do $$ begin
  create trigger on_playlist_notify_pending_purchase_revalidation
  after insert or update of blocknumber on playlists
  for each row execute procedure notify_pending_purchase_revalidation();
exception
  when others then null;
end $$;
