begin;

alter table artist_coins add column if not exists direct_listing boolean not null default false;

comment on column artist_coins.direct_listing is 'Whether this coin was directly listed (bypassing the bonding curve). When true, curveProgress should be 1 and isMigrated should be true.';

commit;
