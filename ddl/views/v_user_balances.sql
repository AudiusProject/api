DROP VIEW IF EXISTS v_user_balances;
CREATE VIEW v_user_balances AS
SELECT
    u.user_id,

    -- Total ETH-side AUDIO balance in wei: primary users.wallet + all linked
    -- chain=eth associated_wallets, summed server-side. eth_wallet_balances
    -- already includes ERC-20 balanceOf + staking + delegation (Multicall3 sum
    -- in eth/indexer/multicall.go), matching the legacy Python
    -- cache_user_balance.py semantic. The IN-list values are LOWER()'d
    -- because eth_wallet_balances stores wallets lowercased (eth-indexer
    -- normalizes via lowerHex()) while associated_wallets historically
    -- accepted mixed-case input. LOWER() is on the *values* being looked
    -- up, not on ewb.wallet, so eth_wallet_balances_pkey still drives the
    -- lookup — no seq scan.
    COALESCE(eth.total_balance, 0)::varchar AS eth_balance,

    -- wAUDIO total for the user — sol_user_balances already pre-aggregates
    -- user_bank PDAs + linked Solana wallets per (user_id, mint), maintained
    -- by handle_sol_claimable_accounts / update_sol_user_balance triggers.
    -- One PK lookup; replaces the two LATERAL subqueries the previous shape
    -- of this view used. wAUDIO base units (8 decimals).
    COALESCE(sub.balance, 0)::varchar AS sol_balance,

    GREATEST(
        COALESCE(eth.updated_at, '1970-01-01'::timestamp),
        COALESCE(sub.updated_at, '1970-01-01'::timestamp)
    ) AS updated_at
FROM users u
LEFT JOIN sol_user_balances sub
    ON sub.user_id = u.user_id
   AND sub.mint = '9LzCMqDgTKYz9Drzqnpgee3SGa89up3a247ypMj2xrqM'
LEFT JOIN LATERAL (
    SELECT SUM(ewb.balance) AS total_balance,
           MAX(ewb.updated_at) AS updated_at
      FROM eth_wallet_balances ewb
     WHERE ewb.wallet IN (
        SELECT LOWER(u.wallet)
        UNION ALL
        SELECT LOWER(aw.wallet)
          FROM associated_wallets aw
         WHERE aw.user_id = u.user_id
           AND aw.chain = 'eth'
           AND aw.is_current = TRUE
           AND aw.is_delete = FALSE
     )
) eth ON TRUE
WHERE u.is_current = TRUE;

COMMENT ON VIEW v_user_balances IS 'Per-user AUDIO/wAUDIO balance totals. One row per current user with eth_balance (wei) and sol_balance (wAUDIO base units, 8 decimals — multiply by 10^10 to compare to wei). eth_balance sums eth_wallet_balances across users.wallet + chain=eth associated_wallets (current, not deleted). sol_balance is sol_user_balances for the wAUDIO mint, already pre-aggregated across user_bank PDAs + linked Solana wallets by handle_sol_claimable_accounts / update_sol_user_balance triggers.';
