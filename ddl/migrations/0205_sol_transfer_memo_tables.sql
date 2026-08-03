BEGIN;
SET LOCAL statement_timeout = 0;

-- Memo-derived transfer classifications. One row per (signature,
-- instruction_index) where the enclosing transaction's memo (or, for
-- Jupiter-touching transactions, the program list) identifies the transfer
-- as a non-bare type — `withdrawal`, `prepare_withdrawal`,
-- `internal_transfer`, or `recover_withdrawal`. Joins by
-- (signature, instruction_index) to either sol_claimable_account_transfers
-- (the first three) or sol_payments (recover_withdrawal). Drives the type
-- derivation in v_token_transactions_history without duplicating the
-- transfer/payment data itself.
CREATE TABLE IF NOT EXISTS sol_transfer_memo_types (
    signature VARCHAR NOT NULL,
    instruction_index INTEGER NOT NULL,
    slot BIGINT NOT NULL,
    memo_type VARCHAR NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    PRIMARY KEY (signature, instruction_index)
);
COMMENT ON TABLE sol_transfer_memo_types IS 'Memo-tagged classifications for claimable_tokens transfers and payment_router routes. memo_type is one of: withdrawal, prepare_withdrawal, internal_transfer, recover_withdrawal.';
CREATE INDEX IF NOT EXISTS sol_transfer_memo_types_slot_idx ON sol_transfer_memo_types (slot);
CREATE INDEX IF NOT EXISTS sol_transfer_memo_types_type_idx ON sol_transfer_memo_types (memo_type);

COMMIT;
