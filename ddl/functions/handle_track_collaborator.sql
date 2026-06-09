-- Notifications for the collaborative-tracks handshake, mirroring
-- handle_manager_request.sql (grants). track_collaborators rows are written by
-- the ETL indexer: a 'pending' row is the owner's invite; a transition to
-- 'accepted' is the collaborator accepting.
--
--   * new pending invite      -> notify the collaborator   ('track_collaborator_invite')
--   * invite becomes accepted  -> notify the inviter/owner   ('track_collaborator_accept')
create or replace function process_track_collaborator_change() returns trigger as $$
begin
    -- A newly created pending invite (created_at = updated_at distinguishes a
    -- fresh insert from a reconciled re-write), or a row resurrected back to
    -- pending: notify the invited collaborator.
    if (TG_OP = 'INSERT' and NEW.status = 'pending' and NEW.created_at = NEW.updated_at) or
       (TG_OP = 'UPDATE' and NEW.status = 'pending' and OLD.status is distinct from 'pending')
    then
        insert into notification
                (blocknumber, user_ids, timestamp, type, specifier, group_id, data)
              values
                (
                  new.blocknumber,
                  array [new.collaborator_user_id],
                  new.updated_at,
                  'track_collaborator_invite',
                  new.invited_by,
                  'track_collaborator_invite:' || 'track_id:' || new.track_id ||
                    ':collaborator_user_id:' || new.collaborator_user_id ||
                    ':inviter_user_id:' || new.invited_by,
                  json_build_object(
                      'track_id', new.track_id,
                      'collaborator_user_id', new.collaborator_user_id,
                      'inviter_user_id', new.invited_by
                    )
                )
              on conflict do nothing;
    -- Invite accepted: notify the inviter (track owner).
    elsif (TG_OP = 'UPDATE' and NEW.status = 'accepted' and OLD.status is distinct from 'accepted') or
          (TG_OP = 'INSERT' and NEW.status = 'accepted')
    then
        insert into notification
                (blocknumber, user_ids, timestamp, type, specifier, group_id, data)
              values
                (
                  new.blocknumber,
                  array [new.invited_by],
                  new.updated_at,
                  'track_collaborator_accept',
                  new.collaborator_user_id,
                  'track_collaborator_accept:' || 'track_id:' || new.track_id ||
                    ':collaborator_user_id:' || new.collaborator_user_id ||
                    ':inviter_user_id:' || new.invited_by,
                  json_build_object(
                      'track_id', new.track_id,
                      'collaborator_user_id', new.collaborator_user_id,
                      'inviter_user_id', new.invited_by
                    )
                )
              on conflict do nothing;
    end if;
    return null;
exception
  when others then
      raise warning 'An error occurred in %: %', tg_name, sqlerrm;
      return null;
end;
$$ language plpgsql;

do $$ begin
  create trigger trigger_track_collaborator_change
  after insert or update on track_collaborators
  for each row execute procedure process_track_collaborator_change();
exception
  when others then null;
end $$;
