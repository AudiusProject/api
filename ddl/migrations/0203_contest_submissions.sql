begin;

-- Backs the new "open contest" event type, where entries are explicit
-- submissions (no parent-track remix relationship). Discovery's entity
-- manager writes a row here when it processes a SubmitToContest action.
-- The existing remix_contest path keeps using the remixes-table join,
-- so this table only carries open_contest entries.
create table if not exists contest_submissions (
    contest_id integer not null,
    track_id integer not null,
    user_id integer not null,
    created_at timestamp without time zone not null default current_timestamp,
    primary key (contest_id, track_id)
);

create index if not exists contest_submissions_track_id_idx
    on contest_submissions using btree (track_id);

create index if not exists contest_submissions_user_id_idx
    on contest_submissions using btree (user_id);

comment on table contest_submissions is 'Tracks submitted to an open_contest event. Keyed by (contest_id, track_id); contest_id references events.event_id.';

commit;
