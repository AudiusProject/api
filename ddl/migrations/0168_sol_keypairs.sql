begin;

create table sol_keypairs (
    id bigint primary key,
    public_key varchar not null,
    private_key varchar not null
);

commit;
