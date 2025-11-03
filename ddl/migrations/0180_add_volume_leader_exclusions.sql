begin;

create table if not exists volume_leader_exclusions (
    address TEXT NOT NULL PRIMARY KEY,
    description TEXT
);

commit;