BEGIN;
SET LOCAL statement_timeout = 0;

-- Backfill sol_transfer_memo_types from the legacy
-- usdc_transactions_history table so v_token_transactions_history can
-- classify historical USDC transfers correctly before the Go indexer has
-- reprocessed them.
--
-- Notification safety: handle_sol_usdc_withdrawal triggers on
-- sol_claimable_account_transfers, NOT on sol_transfer_memo_types — these
-- INSERTs do not fire it. Historical sol_claimable_account_transfers rows
-- were inserted before this PR existed so the trigger did not exist for
-- them either; we are tagging already-inserted rows.
--
-- Coverage:
--   withdrawal / prepare_withdrawal / internal_transfer
--     -> claimable_tokens Transfer, joined via sol_claimable_account_transfers
--   recover_withdrawal
--     -> payment_router Route, joined via sol_payments
--
-- Skipped (intentional):
--   transfer            — no marker needed (bare transfer is the view's default)
--   purchase_content    — already covered by sol_purchases
--   purchase_stripe / coinbase / unknown — out of scope until vendor-memo
--                         capture lands on the Go token indexer

-- Claimable-tokens transfer memo types.
INSERT INTO sol_transfer_memo_types (signature, instruction_index, slot, memo_type)
SELECT cat.signature, cat.instruction_index, cat.slot,
       uth.transaction_type
  FROM usdc_transactions_history uth
  JOIN sol_claimable_account_transfers cat
    ON cat.signature = uth.signature
 WHERE uth.transaction_type IN ('withdrawal', 'prepare_withdrawal', 'internal_transfer')
ON CONFLICT (signature, instruction_index) DO NOTHING;

-- Payment-router recovery memo type. Multiple payment rows can share a
-- signature (one per route destination); DISTINCT keeps the dedup explicit
-- before ON CONFLICT collapses the rest.
INSERT INTO sol_transfer_memo_types (signature, instruction_index, slot, memo_type)
SELECT DISTINCT p.signature, p.instruction_index, p.slot, 'recover_withdrawal'
  FROM usdc_transactions_history uth
  JOIN sol_payments p
    ON p.signature = uth.signature
 WHERE uth.transaction_type = 'recover_withdrawal'
ON CONFLICT (signature, instruction_index) DO NOTHING;

COMMIT;
