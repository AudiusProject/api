create or replace function handle_challenge_disbursement() returns trigger as $$
declare
  reward_manager_tx reward_manager_txs%ROWTYPE;
	existing_notification integer;
	reward_code_exists boolean;
begin

  select * into reward_manager_tx from reward_manager_txs where reward_manager_txs.signature = new.signature limit 1;

  if reward_manager_tx is not null then
		-- check if reward_codes table has an entry where code equals new.challenge_id
		select exists(select 1 from reward_codes where code = new.challenge_id) into reward_code_exists;

		if not reward_code_exists then
			select id into existing_notification
			from notification
			where
			type = 'challenge_reward' and
			new.user_id = any(user_ids) and
			timestamp >= (new.created_at - interval '1 hour')
			limit 1;

			if existing_notification is null then
				-- create a notification for the challenge disbursement
				insert into notification
				(slot, user_ids, timestamp, type, group_id, specifier, data)
				values
				(
					new.slot,
					ARRAY [new.user_id],
					new.created_at,
					'challenge_reward',
					'challenge_reward:' || new.user_id || ':challenge:' || new.challenge_id || ':specifier:' || new.specifier,
					new.user_id,
					json_build_object('specifier', new.specifier, 'challenge_id', new.challenge_id, 'amount', new.amount)
				)
				on conflict do nothing;
			end if;
		end if;
  end if;
  return null;

exception
  when others then
    raise warning 'An error occurred in %: %', tg_name, sqlerrm;
    return null;

end;
$$ language plpgsql;


do $$ begin
  create trigger on_challenge_disbursement
    after insert on challenge_disbursements
    for each row execute procedure handle_challenge_disbursement();
exception
  when others then null;
end $$;


-- Mirror of handle_challenge_disbursement for the new indexer's table.
-- Resolves user_id from user_bank via sol_claimable_accounts -> users.
-- Notification shape and dedupe group_id match the legacy trigger so the two
-- can dual-fire during cutover (the second insert is a no-op via on conflict).
-- Also pg_notify's so the Python ChallengeEventBus can subscribe once the
-- legacy index_rewards_manager Celery task is decommissioned.
create or replace function handle_sol_reward_disbursement() returns trigger as $$
declare
  resolved_user_id integer;
  existing_notification integer;
  reward_code_exists boolean;
begin
  select users.user_id
    into resolved_user_id
    from users
   where users.wallet = new.recipient_eth_address
     and users.is_current = true
   limit 1;

  if resolved_user_id is null then
    return null;
  end if;

  select exists(select 1 from reward_codes where code = new.challenge_id) into reward_code_exists;

  if not reward_code_exists then
    select id into existing_notification
      from notification
     where type = 'challenge_reward'
       and resolved_user_id = any(user_ids)
       and timestamp >= (new.created_at - interval '1 hour')
     limit 1;

    if existing_notification is null then
      insert into notification
        (slot, user_ids, timestamp, type, group_id, specifier, data)
      values
        (
          new.slot,
          ARRAY [resolved_user_id],
          new.created_at,
          'challenge_reward',
          'challenge_reward:' || resolved_user_id || ':challenge:' || new.challenge_id || ':specifier:' || new.specifier,
          resolved_user_id,
          json_build_object('specifier', new.specifier, 'challenge_id', new.challenge_id, 'amount', new.amount)
        )
        on conflict do nothing;
    end if;
  end if;

  perform pg_notify(
    'challenge_disbursed',
    json_build_object(
      'user_id', resolved_user_id,
      'challenge_id', new.challenge_id,
      'specifier', new.specifier,
      'amount', new.amount,
      'signature', new.signature,
      'slot', new.slot
    )::text
  );

  return null;

exception
  when others then
    raise warning 'An error occurred in %: %', tg_name, sqlerrm;
    return null;

end;
$$ language plpgsql;


do $$ begin
  create trigger on_sol_reward_disbursement
    after insert on sol_reward_disbursements
    for each row execute procedure handle_sol_reward_disbursement();
exception
  when others then null;
end $$;
