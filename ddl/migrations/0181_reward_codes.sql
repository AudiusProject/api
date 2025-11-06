begin;

create table if not exists reward_codes (
    code text not null primary key,
    mint text not null,
    reward_address text not null,
    amount bigint not null,
    created_at timestamp without time zone not null default current_timestamp
);

create index if not exists reward_codes_mint_idx on reward_codes using btree (mint);
create index if not exists reward_codes_reward_address_idx on reward_codes using btree (reward_address);

comment on table reward_codes is 'Stores reward codes for distributing coins';
comment on column reward_codes.code is 'Unique code for redemption';
comment on column reward_codes.mint is 'Coin mint address';
comment on column reward_codes.reward_address is 'Address of the reward instance onchain';
comment on column reward_codes.amount is 'Amount of coins to reward';

commit;
