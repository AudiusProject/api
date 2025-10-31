begin;

insert into challenges (id,type,amount,active,step_count,starting_block,weekly_pool,cooldown_days)
values ('dvl','aggregate',10000,true,null,0,2147483647,0) on conflict do nothing;

commit;