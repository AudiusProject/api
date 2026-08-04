-- Remove duplicate is_current rows from users.
--
-- users_pkey is (user_id, txhash), which admits a second is_current row under a
-- different txhash. Five users have one, spread from 2025-06 to 2026-06 — a
-- slow drip, not a one-off. Each pair shares created_at and sits a few blocks
-- apart.
--
-- How is not established. Both indexer create paths reject an existing user
-- (validateUserCreate and migratedUserCreateHandler both call userExists, which
-- tests is_current), so a single writer cannot produce these: its check and its
-- insert share a transaction. A second writer can, since check-then-act is not
-- atomic across transactions. Three of the five pairs put a bare-hex txhash
-- next to a 0x-prefixed one, which fits that reading without proving it.
--
-- Five rows, but the blast radius is not five rows: anything joining an entity
-- to its owner's wallet fans out and silently duplicates every entity those
-- users own — measured at 18 extra tracks and 787 extra follows on a clone.
--
-- This is the backfill half of pkg/etl migration 0035, which adds
-- users_current_uniq_idx to stop it recurring. That index cannot be created
-- while duplicates exist, so this must run first. It does: ddl migrations run
-- in the pre-roll `migrate` Job that every serving Deployment depends on, while
-- the ETL's run at indexer start, after the Job has completed. The delete lives
-- here rather than in the ETL migration so that bumping a Go module can never
-- silently remove rows from this database.
--
-- Deleted rather than demoted: users keeps no versioned history — the indexer
-- writes it in place, and production has zero is_current = false rows — so
-- demoting would leave a category of row nothing reads. Deleting is safe here:
-- no foreign key references users, and its triggers are INSERT (on_user) or
-- INSERT OR UPDATE (trg_users), so neither fires.
--
-- Winner is the highest blocknumber, matching how consumers already pick the
-- live row. Verified against a production clone: in all five cases blocknumber
-- and updated_at agree, and for user 666149592 it keeps is_deactivated = true,
-- the later of that pair's two states. Re-running is a no-op.

BEGIN;
SET LOCAL lock_timeout = '5s';

WITH ranked AS (
    SELECT
        user_id,
        txhash,
        row_number() OVER (
            PARTITION BY user_id
            ORDER BY blocknumber DESC NULLS LAST, updated_at DESC NULLS LAST, txhash DESC
        ) AS rn
    FROM users
    WHERE is_current = true
)
DELETE FROM users u
USING ranked r
WHERE u.user_id = r.user_id
  AND u.txhash = r.txhash
  AND r.rn > 1;

COMMIT;
