BEGIN;

-- Mark queue rows as flushed instead of deleting them.
--
-- The genesis migration chain gets regenerated before it ships (the validator
-- key baked into every block header has to be one the bootstrap node holds), so
-- any row deleted after a successful forward would survive only on a chain that
-- is about to be discarded. Retaining the rows makes new_chain_queue a durable
-- log that can be re-driven onto the rebuilt chain.
ALTER TABLE new_chain_queue ADD COLUMN IF NOT EXISTS flushed_at  timestamptz;
ALTER TABLE new_chain_queue ADD COLUMN IF NOT EXISTS skip_reason text;

COMMENT ON COLUMN new_chain_queue.flushed_at IS 'When this row was forwarded to the new chain, or when it was marked skipped. NULL means pending.';
COMMENT ON COLUMN new_chain_queue.skip_reason IS 'Set when flushed_at was recorded without actually forwarding: ''backfilled'' (covered by the genesis backfill) or ''corrupt'' (tx_data failed to unmarshal).';

-- The flusher only ever reads pending rows, so index those alone. The retained
-- flushed rows stay out of the index and off the hot path no matter how large
-- the table grows.
CREATE INDEX IF NOT EXISTS new_chain_queue_pending_idx
    ON new_chain_queue (id) WHERE flushed_at IS NULL;

COMMIT;
