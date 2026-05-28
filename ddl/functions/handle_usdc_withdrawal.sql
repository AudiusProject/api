create or replace function handle_usdc_withdrawal() returns trigger as $$
DECLARE
    users_row users%ROWTYPE;
    notification_type varchar;
begin

  if new.transaction_type in ('transfer', 'withdrawal') and new.method = 'send' then
    notification_type := 'usdc_' || new.transaction_type;
    -- Fetch the corresponding user based on the wallet
    select into users_row users.*
    from users
    join usdc_user_bank_accounts
      on users.wallet = usdc_user_bank_accounts.ethereum_address
    where usdc_user_bank_accounts.bank_account = new.user_bank;

    -- Insert the new notification
    insert into notification
      (slot, user_ids, timestamp, type, specifier, group_id, data)
    values
      (
        new.slot,
        ARRAY [users_row.user_id],
        new.created_at,
        notification_type,
        users_row.user_id,
        notification_type || ':' || users_row.user_id || ':' || 'signature:' || new.signature,
        json_build_object(
          'user_id', users_row.user_id,
          'user_bank', new.user_bank,
          'signature', new.signature,
          'change', new.change,
          'balance', new.balance,
          'receiver_account', new.tx_metadata
        )
      )
      on conflict do nothing;
  end if;

  return null;
  exception
    when others then
        raise warning 'An error occurred in %: %', tg_name, sqlerrm;
        return null;

end;
$$ language plpgsql;

do $$ begin
  create trigger on_usdc_withdrawal
  after insert on usdc_transactions_history
  for each row execute procedure handle_usdc_withdrawal();
exception
  when others then null;
end $$;

-- Sol-indexer-side equivalent: fires on sol_claimable_account_transfers for
-- USDC transfers out of a user_bank. Mirrors handle_usdc_purchase's parallel
-- handle_sol_purchase pattern. Same notification shape and group_id as the
-- legacy trigger so on-conflict-do-nothing dedupes while both pipelines run
-- side by side; once the Python indexer is decommissioned only this fires.
--
-- Depends on the Go indexer inserting sol_transfer_memo_types BEFORE
-- sol_claimable_account_transfers so the memo lookup below resolves on the
-- first attempt (see solana/indexer/program/claimable_tokens.go).
create or replace function handle_sol_usdc_withdrawal() returns trigger as $$
DECLARE
    users_row users%ROWTYPE;
    mint_addr varchar;
    memo_type_str varchar;
    notification_type varchar;
    change_value bigint;
    balance_value bigint;
    block_ts timestamp;
begin
  -- Resolve sender's mint + user via sol_claimable_accounts. Skip rows
  -- whose from_account isn't a tracked user_bank, and skip non-USDC mints
  -- (AUDIO is not part of the legacy withdrawal-notification surface).
  select sca.mint into mint_addr
    from sol_claimable_accounts sca
   where sca.account = new.from_account
   limit 1;
  select u.* into users_row
    from users u
    join sol_claimable_accounts sca on sca.ethereum_address = u.wallet
   where sca.account = new.from_account
     and u.is_current = true
   limit 1;

  if mint_addr is null or mint_addr not in (
        'EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v', -- prod/stage USDC
        '26Q7gP8UfkDzi7GMFEQxTJaNJ8D2ybCUjex58M5MLu8y'  -- dev USDC
      ) then
    return null;
  end if;

  -- Look up memo type. The legacy trigger only notified for `transfer` and
  -- `withdrawal` — so we skip prepare_withdrawal / internal_transfer /
  -- recover_withdrawal to preserve that behavior.
  select memo_type into memo_type_str
    from sol_transfer_memo_types
   where signature = new.signature and instruction_index = new.instruction_index;

  if memo_type_str is null then
    notification_type := 'usdc_transfer';
  elsif memo_type_str = 'withdrawal' then
    notification_type := 'usdc_withdrawal';
  else
    return null;
  end if;

  -- Pull the per-account balance change (sign + post-balance). Indexer
  -- writes this in common.ProcessBalanceChanges before claimable_tokens
  -- handling, so it's guaranteed to be visible.
  select change, balance, block_timestamp
    into change_value, balance_value, block_ts
    from sol_token_account_balance_changes
   where signature = new.signature
     and account = new.from_account
   limit 1;

  insert into notification
    (slot, user_ids, timestamp, type, specifier, group_id, data)
  values
    (
      new.slot,
      ARRAY[users_row.user_id],
      coalesce(block_ts, now()),
      notification_type,
      users_row.user_id,
      notification_type || ':' || users_row.user_id || ':' || 'signature:' || new.signature,
      json_build_object(
        'user_id', users_row.user_id,
        'user_bank', new.from_account,
        'signature', new.signature,
        'change', change_value,
        'balance', balance_value,
        'receiver_account', new.to_account
      )
    )
    on conflict do nothing;

  return null;
  exception
    when others then
        raise warning 'An error occurred in %: %', tg_name, sqlerrm;
        return null;
end;
$$ language plpgsql;

do $$ begin
  create trigger on_sol_usdc_withdrawal
  after insert on sol_claimable_account_transfers
  for each row execute procedure handle_sol_usdc_withdrawal();
exception
  when others then null;
end $$;
