DROP VIEW IF EXISTS v_user_balances;
CREATE VIEW v_user_balances AS
SELECT
    u.user_id,

    -- ETH primary wallet AUDIO balance, in wei. Each eth_wallet_balances row's
    -- `balance` already includes the wallet's ERC-20 balanceOf + staking +
    -- delegation balances (Multicall3 sum in eth/indexer/multicall.go),
    -- matching the legacy Python cache_user_balance.py semantic for the
    -- `balance` column.
    COALESCE(primary_eth.balance, 0)::varchar AS balance,

    -- Sum of AUDIO balances (in wei) across the user's chain=eth associated
    -- wallets. Same per-wallet semantic as `balance` (includes staking +
    -- delegation).
    COALESCE(linked_eth.total_balance, 0)::varchar AS associated_wallets_balance,

    -- wAUDIO user_bank balance, in base units (8 decimals). Sum across all
    -- the user's claimable USDC/wAUDIO accounts for the wAUDIO mint —
    -- typically a single account per user. Mirrors update_sol_user_balance.sql
    -- which is the canonical source for sol_user_balances rows.
    COALESCE(user_bank_waudio.total_balance, 0)::varchar AS waudio,

    -- Sum of wAUDIO balances across the user's chain=sol associated wallets,
    -- in base units. Same data the update_sol_user_balance.sql function
    -- consults for the associated leg of sol_user_balances.
    COALESCE(linked_sol.total_balance, 0)::varchar AS associated_sol_wallets_balance,

    GREATEST(
        COALESCE(primary_eth.updated_at, '1970-01-01'::timestamp),
        COALESCE(linked_eth.updated_at, '1970-01-01'::timestamp),
        COALESCE(user_bank_waudio.updated_at, '1970-01-01'::timestamp),
        COALESCE(linked_sol.updated_at, '1970-01-01'::timestamp)
    ) AS updated_at
FROM users u
-- ETH primary wallet — eth_wallet_balances.wallet is stored lowercase by the
-- eth-indexer (see lowerHex() in eth/indexer/eth_indexer.go); users.wallet is
-- generally also lowercase but we LOWER() both sides defensively.
LEFT JOIN LATERAL (
    SELECT ewb.balance, ewb.updated_at
      FROM eth_wallet_balances ewb
     WHERE LOWER(ewb.wallet) = LOWER(u.wallet)
     LIMIT 1
) primary_eth ON TRUE
-- ETH associated wallets — sum across all chain=eth linked wallets that have
-- a balance row.
LEFT JOIN LATERAL (
    SELECT SUM(ewb.balance) AS total_balance,
           MAX(ewb.updated_at) AS updated_at
      FROM associated_wallets aw
      JOIN eth_wallet_balances ewb ON LOWER(ewb.wallet) = LOWER(aw.wallet)
     WHERE aw.user_id = u.user_id
       AND aw.chain = 'eth'
       AND aw.is_delete = FALSE
       AND aw.is_current = TRUE
) linked_eth ON TRUE
-- wAUDIO on the user_bank PDA — via sol_claimable_accounts -> sol_token_account_balances.
LEFT JOIN LATERAL (
    SELECT SUM(stab.balance) AS total_balance,
           MAX(stab.updated_at) AS updated_at
      FROM sol_claimable_accounts sca
      JOIN sol_token_account_balances stab ON stab.account = sca.account
     WHERE sca.ethereum_address = u.wallet
       AND sca.mint = '9LzCMqDgTKYz9Drzqnpgee3SGa89up3a247ypMj2xrqM'
       AND stab.mint = '9LzCMqDgTKYz9Drzqnpgee3SGa89up3a247ypMj2xrqM'
) user_bank_waudio ON TRUE
-- wAUDIO on the user's associated Solana wallets — sol_token_account_balances
-- keyed by owner=wallet.
LEFT JOIN LATERAL (
    SELECT SUM(stab.balance) AS total_balance,
           MAX(stab.updated_at) AS updated_at
      FROM associated_wallets aw
      JOIN sol_token_account_balances stab ON stab.owner = aw.wallet
     WHERE aw.user_id = u.user_id
       AND aw.chain = 'sol'
       AND aw.is_delete = FALSE
       AND aw.is_current = TRUE
       AND stab.mint = '9LzCMqDgTKYz9Drzqnpgee3SGa89up3a247ypMj2xrqM'
) linked_sol ON TRUE
WHERE u.is_current = TRUE;

COMMENT ON VIEW v_user_balances IS 'Drop-in replacement for the legacy user_balances table, sourced entirely from indexer-maintained tables (eth_wallet_balances for ETH-side AUDIO+staking+delegation, sol_token_account_balances for wAUDIO). Column shape mirrors user_balances so callers swap with a table-name rename. balance / associated_wallets_balance are in wei; waudio / associated_sol_wallets_balance are in wAUDIO base units (multiply by 10^10 to compare to wei).';
