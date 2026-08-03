-- do a pg_notify
-- n.b. pg_notify has 8kb limit, so for large rows (user, track, playlist) send a known subset of fields (id)
-- for save, repost, aggregate* - the size will never exceed 8kb so it is ok to send the whole row
create or replace function on_new_row() returns trigger as $$
begin
  case TG_TABLE_NAME
    when 'tracks' then
      PERFORM pg_notify(TG_TABLE_NAME, json_build_object('track_id', new.track_id, 'updated_at', new.updated_at, 'created_at', new.created_at, 'blocknumber', new.blocknumber)::text);
    when 'users' then
      PERFORM pg_notify(TG_TABLE_NAME, json_build_object('user_id', new.user_id, 'blocknumber', new.blocknumber)::text);
      -- Dedicated verification-transition channel.
      --
      -- The Go ETL writes `users` in place (single is_current row per user; it
      -- bumps blocknumber rather than appending a versioned row), so consumers
      -- can no longer reconstruct the "previous" verification state from the
      -- `users` table or from revert_blocks. But because the update is in place,
      -- this AFTER trigger's OLD row holds the true pre-update state. Emit on a
      -- separate channel only for the genuine false -> true transition (or a
      -- brand-new row that arrives already verified) so downstream listeners
      -- (e.g. the verified-notifications Slack bot) fire exactly once instead of
      -- on every profile edit by an already-verified user.
      if new.is_verified and (TG_OP = 'INSERT' or not coalesce(old.is_verified, false)) then
        PERFORM pg_notify('user_verified', json_build_object('user_id', new.user_id, 'blocknumber', new.blocknumber)::text);
      end if;
    when 'playlists' then
      PERFORM pg_notify(TG_TABLE_NAME, json_build_object('playlist_id', new.playlist_id)::text);
    else
      PERFORM pg_notify(TG_TABLE_NAME, to_json(new)::text);
  end case;
  return null;
end;
$$ language plpgsql;

-- register trigger for an insert / update on the following tables
do $$
declare
  tbl text;
  tbls text[] := ARRAY[
    'aggregate_plays',
    'aggregate_user',
    'follows',
    'playlists',
    'reposts',
    'saves',
    'shares',
    'tracks',
    'users',
    'usdc_purchases',
    'artist_coins'
  ];
begin
  FOREACH tbl IN ARRAY tbls
  loop
    RAISE NOTICE 'creating trg_%', tbl;
    EXECUTE 'drop trigger if exists trg_' ||tbl|| ' on ' ||tbl|| ';';
    EXECUTE 'create trigger trg_' ||tbl|| ' after insert or update on ' ||tbl|| ' for each row execute procedure on_new_row();';
  end loop;
end $$;