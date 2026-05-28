BEGIN;
SET LOCAL statement_timeout = 0;

-- Canonicalize chain='eth' associated_wallets.wallet to lowercase so it
-- matches everything else in the system that stores ETH addresses
-- (eth_wallet_balances, users.wallet, eth-indexer reads). Ethereum
-- addresses are case-insensitive at the protocol level (EIP-55 mixed case
-- is just a checksum), so this is purely a normalization — semantically
-- equivalent, but unblocks PK-driven joins that were previously failing
-- the case match.
--
-- Solana wallets are explicitly excluded: base58 IS case-sensitive, so
-- LOWER()ing them would corrupt them.

-- Step 1: dedup. The (user_id, wallet, chain) UNIQUE constraint means
-- '0xAbC...' and '0xabc...' currently count as distinct rows for the same
-- user; after canonicalization they'd collide. Keep one row per group with
-- this preference (most "live", then most recent):
--   * is_delete = FALSE before is_delete = TRUE
--   * higher id before lower id
DELETE FROM associated_wallets
 WHERE id IN (
   SELECT id
     FROM (
       SELECT id,
              ROW_NUMBER() OVER (
                PARTITION BY user_id, LOWER(wallet)
                ORDER BY is_delete ASC, id DESC
              ) AS rn
         FROM associated_wallets
        WHERE chain = 'eth'
     ) ranked
    WHERE rn > 1
 );

-- Step 2: canonicalize remaining rows. The handle_associated_wallets
-- trigger only re-runs update_sol_user_balance_mint on INSERT, DELETE, and
-- is_delete changes — a pure wallet UPDATE doesn't cascade, so the
-- trigger stays enabled and this is cheap.
UPDATE associated_wallets
   SET wallet = LOWER(wallet)
 WHERE chain = 'eth'
   AND wallet <> LOWER(wallet);

-- Step 3: prevent future mixed-case inserts. BEFORE INSERT/UPDATE trigger
-- silently lowercases incoming chain='eth' wallets so callers that still
-- send EIP-55 mixed case don't reintroduce the divergence. Scoped to
-- 'eth' so Solana wallets (case-sensitive base58) are untouched.
CREATE OR REPLACE FUNCTION canonicalize_associated_wallet()
RETURNS TRIGGER AS $$
BEGIN
    IF NEW.chain = 'eth' AND NEW.wallet IS NOT NULL THEN
        NEW.wallet := LOWER(NEW.wallet);
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS before_associated_wallets_canonicalize ON associated_wallets;
CREATE TRIGGER before_associated_wallets_canonicalize
    BEFORE INSERT OR UPDATE ON associated_wallets
    FOR EACH ROW EXECUTE PROCEDURE canonicalize_associated_wallet();
COMMENT ON TRIGGER before_associated_wallets_canonicalize ON associated_wallets IS
    'Lowercases chain=eth wallets on insert/update so eth_wallet_balances joins (PK on lowercase) hit without case mismatches. Solana wallets are left alone — base58 is case-sensitive.';

COMMIT;
