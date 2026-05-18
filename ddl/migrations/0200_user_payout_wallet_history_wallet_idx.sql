BEGIN;

-- v_usdc_purchases looks up "which user owned this Solana payout wallet at
-- purchase time" by joining user_payout_wallet_history. The PK is
-- (user_id, block_timestamp), which is the wrong direction for that lookup
-- (we know the wallet, want the user). Add a covering index on
-- (spl_usdc_payout_wallet, block_timestamp) so the LATERAL subquery can
-- index-scan + bounded backward to find the row applicable at sp.created_at.
CREATE INDEX IF NOT EXISTS user_payout_wallet_history_wallet_idx
    ON user_payout_wallet_history (spl_usdc_payout_wallet, block_timestamp);

COMMIT;
