begin;

alter table reward_codes add column if not exists remaining_uses integer not null default 1;
alter table reward_codes alter column is_used drop not null;

comment on column reward_codes.remaining_uses is 'Number of times the code can still be redeemed';

commit;
