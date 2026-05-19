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

BEGIN;

-- Build both indexes inside the migration transaction (non-CONCURRENTLY) so
-- the writes are atomic with the backfill and not subject to virtualxid waits
-- against concurrent transactions on this DB (notably the legacy Python
-- index_rewards_manager Celery task on discovery-provider, which keeps long
-- challenge_disbursements transactions open and can make CONCURRENTLY hang
-- indefinitely). Regular CREATE INDEX takes a ShareLock on the target table:
-- writes are blocked for the duration of the build, but both target tables
-- have light write load (reward_manager EvaluateAttestations and claimable
-- token creations are sparse on-chain) and the build itself completes in
-- seconds at current row counts.
--
-- Both indexes pay off well beyond this migration: the first lets the dedup
-- LEFT JOIN on (challenge_id, specifier) use an index instead of a sequential
-- scan; the second lets the live reward_manager indexer (and the CTE below)
-- resolve a user's current claimable account in O(log n) rather than scanning
-- the table.

CREATE INDEX IF NOT EXISTS
    sol_reward_disbursements_challenge_specifier_idx
    ON sol_reward_disbursements (challenge_id, specifier);

CREATE INDEX IF NOT EXISTS
    sol_claimable_accounts_eth_mint_slot_idx
    ON sol_claimable_accounts (ethereum_address, mint, slot DESC);

-- Skip the on_sol_reward_disbursement trigger for this transaction. The
-- trigger fires per-row to create challenge_reward notifications and a
-- pg_notify announcement for the Python ChallengeEventBus. For a one-shot
-- backfill of months-old disbursements, those notifications would be
-- both noisy (~29k user-facing pushes for historical rewards) and slow
-- (extra SELECTs and an INSERT per row). SET LOCAL scopes this to the
-- transaction so concurrent indexer writes still fire the trigger normally.
SET LOCAL session_replication_role = replica;

-- Pre-compute the current AUDIO claimable account per wallet in one indexed
-- scan rather than re-running the LATERAL subquery per challenge_disbursements
-- row. MATERIALIZED forces a one-time evaluation that the planner can hash-
-- join against, instead of inlining the CTE into the main query.
WITH user_banks AS MATERIALIZED (
    SELECT DISTINCT ON (ethereum_address)
        ethereum_address,
        account
    FROM sol_claimable_accounts
    WHERE mint = '9LzCMqDgTKYz9Drzqnpgee3SGa89up3a247ypMj2xrqM'
    ORDER BY ethereum_address, slot DESC
)
INSERT INTO sol_reward_disbursements
    (signature, instruction_index, amount, slot, user_bank, challenge_id, specifier, recipient_eth_address, created_at)
SELECT
    cd.signature,
    0 AS instruction_index,
    cd.amount::bigint,
    cd.slot,
    ub.account AS user_bank,
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
JOIN user_banks ub
       ON ub.ethereum_address = u.wallet
WHERE rd.signature IS NULL
ON CONFLICT (signature, instruction_index) DO NOTHING;

COMMIT;
