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

-- Restore the indexes the legacy challenge_disbursements table had so the API
-- routes that filter/sort by user (via recipient_eth_address) and created_at
-- don't degenerate to seq scans through v_challenge_disbursements.
CREATE INDEX IF NOT EXISTS sol_reward_disbursements_recipient_eth_address_idx
    ON sol_reward_disbursements (recipient_eth_address);

CREATE INDEX IF NOT EXISTS sol_reward_disbursements_created_at_idx
    ON sol_reward_disbursements (created_at);

COMMIT;
