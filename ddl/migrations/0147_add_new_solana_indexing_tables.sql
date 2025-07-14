BEGIN;

CREATE TABLE IF NOT EXISTS sol_claimable_accounts (
	signature VARCHAR NOT NULL,
	instruction_index INT NOT NULL,
	slot BIGINT NOT NULL,
	
	mint VARCHAR NOT NULL,
	ethereum_address VARCHAR NOT NULL,
	bank_account VARCHAR NOT NULL,
	
	PRIMARY KEY (signature, instruction_index)
);
CREATE INDEX IF NOT EXISTS sol_claimable_accounts_slot_idx ON sol_claimable_accounts (slot);
CREATE INDEX IF NOT EXISTS sol_claimable_accounts_mint_idx ON sol_claimable_accounts (mint);
CREATE INDEX IF NOT EXISTS sol_claimable_accounts_ethereum_address_idx ON sol_claimable_accounts (ethereum_address);
CREATE INDEX IF NOT EXISTS sol_claimable_accounts_bank_account_idx ON sol_claimable_accounts (bank_account);


CREATE TABLE IF NOT EXISTS sol_claimable_account_transfers (
	signature VARCHAR NOT NULL,
	instruction_index INT NOT NULL,
	amount BIGINT NOT NULL,
	slot BIGINT NOT NULL,
	
	from_account VARCHAR NOT NULL,
	to_account VARCHAR NOT NULL,
	sender_eth_address VARCHAR NOT NULL,
	
	PRIMARY KEY (signature, instruction_index)
);
CREATE INDEX IF NOT EXISTS sol_claimable_account_transfers_slot_idx ON sol_claimable_account_transfers (slot);
CREATE INDEX IF NOT EXISTS sol_claimable_account_transfers_from_idx ON sol_claimable_account_transfers (from_account);
CREATE INDEX IF NOT EXISTS sol_claimable_account_transfers_to_idx ON sol_claimable_account_transfers (to_account);
CREATE INDEX IF NOT EXISTS sol_claimable_account_transfers_sender_eth_address_idx ON sol_claimable_account_transfers (sender_eth_address);


CREATE TABLE IF NOT EXISTS sol_reward_disbursements (
	signature VARCHAR NOT NULL,
	instruction_index INT NOT NULL,
	amount BIGINT NOT NULL,
	slot BIGINT NOT NULL,
	
	user_bank VARCHAR NOT NULL,
	challenge_id VARCHAR NOT NULL,
	specifier VARCHAR NOT NULL,
	
	PRIMARY KEY (signature, instruction_index)
);
CREATE INDEX IF NOT EXISTS sol_reward_disbursements_slot_idx ON sol_reward_disbursements (slot);
CREATE INDEX IF NOT EXISTS sol_reward_disbursements_user_bank_idx ON sol_reward_disbursements (user_bank);
CREATE INDEX IF NOT EXISTS sol_reward_disbursements_challenge_idx ON sol_reward_disbursements (challenge_id, specifier);


CREATE TABLE IF NOT EXISTS sol_payments (
	signature VARCHAR NOT NULL,
	instruction_index INT NOT NULL,
	amount BIGINT NOT NULL,
	slot BIGINT NOT NULL,
	
	route_index INT NOT NULL,
	to_account VARCHAR NOT NULL,
	
	
	PRIMARY KEY (signature, instruction_index, route_index)
);
CREATE INDEX IF NOT EXISTS sol_payments_slot_idx ON sol_payments (slot);
CREATE INDEX IF NOT EXISTS sol_payments_to_idx ON sol_payments (to_account);


CREATE TABLE IF NOT EXISTS sol_purchases (
	signature VARCHAR NOT NULL,
	instruction_index INT NOT NULL,
	amount BIGINT NOT NULL,
	slot BIGINT NOT NULL,
	
	from_account VARCHAR NOT NULL,

	content_type VARCHAR NOT NULL,
	content_id INT NOT NULL,
	buyer_user_id INT NOT NULL,
	access_type VARCHAR NOT NULL,
	valid_after_blocknumber BIGINT NOT NULL,
	is_valid BOOLEAN,
	
	city VARCHAR,
	region VARCHAR,
	country VARCHAR,
	
	PRIMARY KEY (signature, instruction_index)
);
CREATE INDEX IF NOT EXISTS sol_purchases_slot_idx ON sol_purchases (slot);
CREATE INDEX IF NOT EXISTS sol_purchases_from_account_idx ON sol_purchases (from_account);
CREATE INDEX IF NOT EXISTS sol_purchases_buyer_user_id_idx ON sol_purchases (buyer_user_id);
CREATE INDEX IF NOT EXISTS sol_purchases_content_idx ON sol_purchases (content_id, content_type, access_type);
CREATE INDEX IF NOT EXISTS sol_purchases_valid_idx ON sol_purchases (is_valid, valid_after_blocknumber);


CREATE TABLE IF NOT EXISTS sol_swaps (
	signature VARCHAR NOT NULL,
	instruction_index INT NOT NULL,
	slot BIGINT NOT NULL,
	
	from_mint VARCHAR NOT NULL,
	from_account VARCHAR NOT NULL,
	from_amount BIGINT NOT NULL,
	
	to_mint VARCHAR NOT NULL,
	to_account VARCHAR NOT NULL,
	to_amount BIGINT NOT NULL,
	
	PRIMARY KEY (signature, instruction_index)
);
CREATE INDEX IF NOT EXISTS sol_swaps_slot_idx ON sol_swaps (slot);
CREATE INDEX IF NOT EXISTS sol_swaps_from_mint_idx ON sol_swaps (from_mint);
CREATE INDEX IF NOT EXISTS sol_swaps_from_account_idx ON sol_swaps (from_account);
CREATE INDEX IF NOT EXISTS sol_swaps_to_mint_idx ON sol_swaps (to_mint);
CREATE INDEX IF NOT EXISTS sol_swaps_to_account_idx ON sol_swaps (to_account);


CREATE TABLE IF NOT EXISTS sol_token_transfers (
	signature VARCHAR NOT NULL,
	instruction_index INT NOT NULL,
	amount BIGINT NOT NULL,
	slot BIGINT NOT NULL,
	
	from_account VARCHAR NOT NULL,
	to_account VARCHAR NOT NULL
	
	PRIMARY KEY (signature, instruction_index)
);
CREATE INDEX IF NOT EXISTS sol_token_transfers_slot_idx ON sol_token_transfers (slot);
CREATE INDEX IF NOT EXISTS sol_token_transfers_from_account_idx ON sol_token_transfers (from_account);
CREATE INDEX IF NOT EXISTS sol_token_transfers_to_account_idx ON sol_token_transfers (to_account);

COMMIT;