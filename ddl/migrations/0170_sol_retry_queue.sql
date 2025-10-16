CREATE TABLE IF NOT EXISTS sol_retry_queue (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    indexer TEXT NOT NULL,
    update_message JSONB NOT NULL,
    error TEXT NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
COMMENT ON TABLE sol_retry_queue IS 'Queue for retrying failed indexer updates.';
COMMENT ON COLUMN sol_retry_queue.indexer IS 'The name of the indexer that failed (e.g., token_indexer, damm_v2_indexer).';
COMMENT ON COLUMN sol_retry_queue.update_message IS 'The JSONB update data that failed to process.';
COMMENT ON COLUMN sol_retry_queue.error IS 'The error message from the failure.';
COMMENT ON COLUMN sol_retry_queue.created_at IS 'The timestamp when the retry entry was created.';
COMMENT ON COLUMN sol_retry_queue.updated_at IS 'The timestamp when the retry entry was last updated.';

ALTER TABLE sol_slot_checkpoints ADD COLUMN IF NOT EXISTS name TEXT;
COMMENT ON COLUMN sol_slot_checkpoints.name IS 'The name of the indexer this checkpoint is for (e.g., token_indexer, damm_v2_indexer).';

DROP TABLE IF EXISTS sol_unprocessed_txs;