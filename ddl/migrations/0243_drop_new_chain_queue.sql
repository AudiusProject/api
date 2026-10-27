-- Retire new_chain_queue. The API no longer enqueues or flushes writes for the
-- new chain; a standalone mirror in go-openaudio replays the old chain instead.
BEGIN;

DROP TABLE IF EXISTS new_chain_queue;

COMMIT;
