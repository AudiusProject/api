BEGIN;

-- Parity with the legacy challenge_disbursements.created_at column. The Go indexer
-- writes rows close to on-chain time, so DEFAULT NOW() is acceptable for new rows;
-- backfilled rows are corrected from the legacy table below.
ALTER TABLE sol_reward_disbursements
    ADD COLUMN IF NOT EXISTS created_at TIMESTAMP DEFAULT NOW();

UPDATE sol_reward_disbursements rd
   SET created_at = cd.created_at
  FROM challenge_disbursements cd
 WHERE cd.signature = rd.signature
   AND cd.created_at IS NOT NULL
   AND (rd.created_at IS NULL OR rd.created_at > cd.created_at);

COMMIT;
