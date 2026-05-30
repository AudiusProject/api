BEGIN;
SET LOCAL statement_timeout = 0;

-- Pre-aggregated AUDIO (ETH-side, in wei) balance per user — the ETH analog of
-- sol_user_balances. Replaces the per-user LATERAL aggregate that
-- v_user_balances ran over eth_wallet_balances on every read (the GetUsers hot
-- path): summing a user's wallets live, per row, made GetUsers scale with the
-- whole user base whenever the planner materialized the view instead of
-- parameterizing it. This table turns that into a single PK lookup, exactly
-- like the sol side.
--
-- Kept fresh by triggers:
--   * handle_eth_wallet_balance_change  (eth_wallet_balances writes)
--   * handle_associated_wallets         (chain=eth wallet link / unlink)
-- and recomputed for one user via update_eth_user_balance(user_id).
CREATE TABLE IF NOT EXISTS eth_user_balances (
    user_id    INT PRIMARY KEY,
    balance    NUMERIC NOT NULL DEFAULT 0,
    updated_at TIMESTAMP NOT NULL DEFAULT now(),
    created_at TIMESTAMP NOT NULL DEFAULT now()
);
COMMENT ON TABLE eth_user_balances IS 'Per-user AUDIO ERC-20 balance (wei), summed across users.wallet + chain=eth associated_wallets. Pre-aggregated mirror of eth_wallet_balances, maintained by triggers (handle_eth_wallet_balance_change / handle_associated_wallets) and recomputed by update_eth_user_balance(user_id). ETH-side analog of sol_user_balances.';

-- Backfill from eth_wallet_balances. eth_wallet_balances.wallet is lowercase
-- hex (PK); associated_wallets.wallet is already lowercased for chain=eth
-- (migration 0207), and users.wallet is lowered to match. UNION ALL (not
-- UNION) mirrors the previous view semantics exactly — if a wallet is both a
-- user's primary and one of their own linked wallets it counts the same way it
-- did before.
INSERT INTO eth_user_balances (user_id, balance, updated_at)
SELECT user_id, SUM(balance), MAX(updated_at)
FROM (
    SELECT u.user_id, ewb.balance, ewb.updated_at
      FROM users u
      JOIN eth_wallet_balances ewb ON ewb.wallet = LOWER(u.wallet)
     WHERE u.is_current = TRUE

    UNION ALL

    SELECT aw.user_id, ewb.balance, ewb.updated_at
      FROM associated_wallets aw
      JOIN eth_wallet_balances ewb ON ewb.wallet = LOWER(aw.wallet)
     WHERE aw.chain = 'eth'
       AND aw.is_current = TRUE
       AND aw.is_delete = FALSE
) b
GROUP BY user_id
ON CONFLICT (user_id) DO NOTHING;

COMMIT;
