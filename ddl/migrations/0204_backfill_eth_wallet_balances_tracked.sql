BEGIN;
SET LOCAL statement_timeout = 0;

-- Seed eth_wallet_balances with every tracked Ethereum wallet at
-- updated_at = epoch so the eth-indexer's stale-refresh sweep
-- (eth/indexer/eth_indexer.go:selectStaleWallets, ORDER BY updated_at ASC)
-- picks them up oldest-first and reads the real on-chain balance via
-- Multicall3 (balanceOf + totalStakedFor + getTotalDelegatorStake).
--
-- Without this seed, long-time stakers / delegators who haven't moved AUDIO
-- since the eth-indexer started watching are invisible to it — the sweep
-- can only refresh rows that already exist in eth_wallet_balances, and the
-- live Transfer subscription never sees them. Confirmed via diagnostics on
-- the top 25 legacy balances: 9 users had multi-hundred-thousand-AUDIO gaps
-- entirely from associated_wallets chain=eth wallets that had zero coverage.
--
-- Idempotent: ON CONFLICT DO NOTHING preserves any existing row's balance
-- and updated_at, so re-running this migration is safe and so is a fresh
-- run after rows have been refreshed.

INSERT INTO eth_wallet_balances (wallet, balance, updated_at)
SELECT wallet, 0, '1970-01-01'::timestamp
FROM (
    SELECT LOWER(wallet) AS wallet
      FROM users
     WHERE is_current = TRUE
       AND wallet IS NOT NULL
       AND wallet <> ''
    UNION
    SELECT LOWER(wallet) AS wallet
      FROM associated_wallets
     WHERE chain = 'eth'
       AND is_delete = FALSE
       AND wallet IS NOT NULL
       AND wallet <> ''
) t
ON CONFLICT (wallet) DO NOTHING;

COMMIT;
