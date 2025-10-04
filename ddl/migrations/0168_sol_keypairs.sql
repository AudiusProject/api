begin;

create table sol_keypairs (
    public_key varchar primary key,
    private_key bytea not null
);

commit;
