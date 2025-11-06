begin;

alter table reward_codes add column if not exists is_used boolean not null default false;

comment on column reward_codes.is_used is 'Whether the code has been redeemed';

commit;

