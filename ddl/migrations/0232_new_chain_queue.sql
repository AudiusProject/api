BEGIN;

CREATE TABLE IF NOT EXISTS new_chain_queue (
    id              bigserial PRIMARY KEY,
    created_at      timestamptz NOT NULL DEFAULT now(),
    tx_data         bytea NOT NULL,
    confirmed_block bigint
);

COMMENT ON TABLE new_chain_queue IS 'Queue of ManageEntity transactions to be forwarded to the new Core chain (audius-mainnet-v2) during genesis migration.';
COMMENT ON COLUMN new_chain_queue.tx_data IS 'Protobuf-serialized ManageEntityLegacy message.';
COMMENT ON COLUMN new_chain_queue.confirmed_block IS 'Block height on the old chain where this transaction was confirmed. NULL if confirmation was not recorded (e.g. relay restart).';

CREATE INDEX IF NOT EXISTS new_chain_queue_confirmed_block_idx ON new_chain_queue (confirmed_block);

COMMIT;
