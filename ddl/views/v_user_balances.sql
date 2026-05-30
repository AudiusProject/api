DROP VIEW IF EXISTS v_user_balances;
CREATE VIEW v_user_balances AS
SELECT
    u.user_id,

    -- Total ETH-side AUDIO balance in wei: primary users.wallet + all linked
    -- chain=eth associated_wallets, pre-aggregated into eth_user_balances and
    -- maintained by handle_eth_wallet_balance_change / handle_associated_wallets
    -- triggers (recomputed via update_eth_user_balance). A single PK lookup —
    -- it replaces the per-user LATERAL aggregate over eth_wallet_balances that
    -- previously made GetUsers scan the whole user base whenever the planner
    -- materialized this view instead of parameterizing it.
    COALESCE(eub.balance, 0)::varchar AS eth_balance,

    -- wAUDIO total for the user — sol_user_balances already pre-aggregates
    -- user_bank PDAs + linked Solana wallets per (user_id, mint), maintained
    -- by handle_sol_claimable_accounts / update_sol_user_balance triggers.
    -- One PK lookup. wAUDIO base units (8 decimals).
    COALESCE(sub.balance, 0)::varchar AS sol_balance,

    GREATEST(
        COALESCE(eub.updated_at, '1970-01-01'::timestamp),
        COALESCE(sub.updated_at, '1970-01-01'::timestamp)
    ) AS updated_at
FROM users u
LEFT JOIN sol_user_balances sub
    ON sub.user_id = u.user_id
   AND sub.mint = '9LzCMqDgTKYz9Drzqnpgee3SGa89up3a247ypMj2xrqM'
LEFT JOIN eth_user_balances eub
    ON eub.user_id = u.user_id
WHERE u.is_current = TRUE;

COMMENT ON VIEW v_user_balances IS 'Per-user AUDIO/wAUDIO balance totals. One row per current user with eth_balance (wei) and sol_balance (wAUDIO base units, 8 decimals — multiply by 10^10 to compare to wei). eth_balance is eth_user_balances (pre-aggregated across users.wallet + chain=eth associated_wallets, maintained by handle_eth_wallet_balance_change / handle_associated_wallets). sol_balance is sol_user_balances for the wAUDIO mint, pre-aggregated across user_bank PDAs + linked Solana wallets by handle_sol_claimable_accounts / update_sol_user_balance triggers.';
