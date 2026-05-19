BEGIN;

-- One-shot recovery of challenge_disbursements rows that never made it into
-- sol_reward_disbursements. Two historical loss sources contributed:
--
--   1. Migration 0152's INNER JOIN on user_bank_accounts dropped rows whose
--      authoring user no longer had a current user_bank_accounts entry. Those
--      rows were silently excluded from the original backfill.
--
--   2. Before the indexer.go:113 swallowed-error fix, the live Go indexer
--      could drop reward_manager EvaluateAttestations transactions whenever
--      ProcessTransaction returned an error, without surfacing the failure
--      to the retry queue or pausing the slot checkpoint.
--
-- This migration recovers only the rows we can reconstruct from current
-- relational state: a current users row plus an indexed AUDIO sol_claimable
-- account. Rows whose user record no longer exists are intentionally skipped;
-- they would need on-chain signature replay (via program.Indexer) to recover.
INSERT INTO sol_reward_disbursements
    (signature, instruction_index, amount, slot, user_bank, challenge_id, specifier, recipient_eth_address, created_at)
SELECT
    cd.signature,
    0 AS instruction_index,
    cd.amount::bigint,
    cd.slot,
    sca.account AS user_bank,
    cd.challenge_id,
    cd.specifier,
    LOWER(u.wallet) AS recipient_eth_address,
    cd.created_at
FROM challenge_disbursements cd
LEFT JOIN sol_reward_disbursements rd
       ON rd.challenge_id = cd.challenge_id
      AND rd.specifier    = cd.specifier
JOIN users u
       ON u.user_id    = cd.user_id
      AND u.is_current = TRUE
JOIN LATERAL (
    -- A user can have multiple sol_claimable_accounts rows (one per on-chain
    -- Create instruction over time). Pick the latest as the active user_bank.
    SELECT account
    FROM sol_claimable_accounts
    WHERE ethereum_address = u.wallet
      AND mint = '9LzCMqDgTKYz9Drzqnpgee3SGa89up3a247ypMj2xrqM'
    ORDER BY slot DESC
    LIMIT 1
) sca ON TRUE
WHERE rd.signature IS NULL
ON CONFLICT (signature, instruction_index) DO NOTHING;

COMMIT;
