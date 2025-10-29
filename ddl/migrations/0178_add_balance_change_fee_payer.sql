begin;

ALTER TABLE sol_token_account_balance_changes
ADD COLUMN IF NOT EXISTS fee_payer VARCHAR;

COMMENT ON COLUMN sol_token_account_balance_changes.fee_payer IS 'The public key of the account that paid the fee for the transaction.';

COMMIT;