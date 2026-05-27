DROP VIEW IF EXISTS v_user_balances;
CREATE VIEW v_user_balances AS
SELECT
    u.user_id,

    -- ETH primary wallet AUDIO balance, in wei. eth_wallet_balances.balance
    -- already includes ERC-20 balanceOf + staking + delegation (Multicall3
    -- sum in eth/indexer/multicall.go), matching the legacy Python
    -- cache_user_balance.py semantic. PK lookup on eth_wallet_balances.wallet
    -- — no LOWER() on either side because the eth-indexer normalizes via
    -- lowerHex() and users.wallet is already lowercase, and LOWER() on the
    -- join key would force a seq scan instead of using the PK.
    COALESCE(ewb.balance, 0)::varchar AS balance,

    -- Sum across the user's chain=eth linked wallets. Driven by the user_id
    -- index on associated_wallets, then PK lookups on eth_wallet_balances.
    COALESCE(linked_eth.total_balance, 0)::varchar AS associated_wallets_balance,

    -- wAUDIO total for the user — sol_user_balances already pre-aggregates
    -- user_bank PDAs + linked Solana wallets per (user_id, mint), maintained
    -- by handle_sol_claimable_accounts / update_sol_user_balance triggers.
    -- One PK lookup; replaces the two LATERAL subqueries the previous shape
    -- of this view used.
    COALESCE(sub.balance, 0)::varchar AS waudio,

    -- Legacy split: waudio (user_bank) vs associated_sol_wallets_balance
    -- (linked Solana wallets). sol_user_balances rolls both into a single
    -- balance per mint, so we surface the full total under `waudio` and
    -- leave this column zero. Downstream total_balance / total_audio_balance
    -- computations sum the two fields, so totals are unchanged.
    '0'::varchar AS associated_sol_wallets_balance,

    GREATEST(
        COALESCE(ewb.updated_at, '1970-01-01'::timestamp),
        COALESCE(linked_eth.updated_at, '1970-01-01'::timestamp),
        COALESCE(sub.updated_at, '1970-01-01'::timestamp)
    ) AS updated_at
FROM users u
LEFT JOIN eth_wallet_balances ewb ON ewb.wallet = u.wallet
LEFT JOIN sol_user_balances sub
    ON sub.user_id = u.user_id
   AND sub.mint = '9LzCMqDgTKYz9Drzqnpgee3SGa89up3a247ypMj2xrqM'
LEFT JOIN LATERAL (
    SELECT SUM(ewb2.balance) AS total_balance,
           MAX(ewb2.updated_at) AS updated_at
      FROM associated_wallets aw
      JOIN eth_wallet_balances ewb2 ON ewb2.wallet = aw.wallet
     WHERE aw.user_id = u.user_id
       AND aw.chain = 'eth'
       AND aw.is_current = TRUE
       AND aw.is_delete = FALSE
) linked_eth ON TRUE
WHERE u.is_current = TRUE;

COMMENT ON VIEW v_user_balances IS 'Drop-in replacement for the legacy user_balances table. ETH-side (balance / associated_wallets_balance) sourced from eth_wallet_balances; wAUDIO total sourced from sol_user_balances (pre-aggregated by triggers, rolls user_bank + linked sol wallets into one row per user/mint). associated_sol_wallets_balance is always 0 — the legacy user_bank-vs-linked split is collapsed into waudio; downstream total_balance computations are unchanged. All balances stored as varchar to match the legacy column types: ETH columns in wei, waudio in wAUDIO base units (multiply by 10^10 to compare to wei).';
