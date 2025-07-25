-- user_bank_accounts => sol_claimable_accounts ~1min
INSERT INTO sol_claimable_accounts 
	(signature, instruction_index, slot, mint, ethereum_address, account)
SELECT
	user_bank_accounts.signature,
	0 AS instruction_index,
	slot,
	'9LzCMqDgTKYz9Drzqnpgee3SGa89up3a247ypMj2xrqM' AS mint,
	ethereum_address,
	bank_account AS account
FROM user_bank_accounts
JOIN user_bank_txs
	ON user_bank_accounts.signature = user_bank_txs.signature
ON CONFLICT DO NOTHING;

-- user_tips => sol_claimable_account_transfers ~30s
INSERT INTO sol_claimable_account_transfers
	(signature, instruction_index, amount, slot, from_account, to_account, sender_eth_address)
SELECT
	user_tips.signature,
	1 AS instruction_index,
	user_tips.amount,
	user_tips.slot,
	from_user_banks.bank_account AS from_account,
	to_user_banks.bank_account AS to_account,
	from_users.wallet AS sender_eth_address
FROM user_tips
JOIN users AS from_users 
	ON from_users.user_id = user_tips.sender_user_id
JOIN user_bank_accounts AS from_user_banks
	ON from_users.wallet = from_user_banks.ethereum_address
JOIN users AS to_users
	ON to_users.user_id = user_tips.receiver_user_id
JOIN user_bank_accounts AS to_user_banks
	ON to_users.wallet = to_user_banks.ethereum_address
ON CONFLICT DO NOTHING;


-- audio_transactions_history => sol_token_account_balance_changes ~1min
INSERT INTO sol_token_account_balance_changes
	(signature, mint, owner, account, change, balance, slot, updated_at, created_at, block_timestamp)
SELECT
	audio_transactions_history.signature,
	'9LzCMqDgTKYz9Drzqnpgee3SGa89up3a247ypMj2xrqM' AS mint,
	'5ZiE3vAkrdXBgyFL7KqG3RoEGBws4CjRcXVbABDLZTgx' AS owner, -- claimable_tokens program PDA
	audio_transactions_history.user_bank AS account,
	change,
	balance,
	slot,
	updated_at,
	created_at,
	transaction_created_at AS block_timestamp
FROM audio_transactions_history;	