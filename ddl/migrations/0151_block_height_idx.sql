begin;

-- Not used
DROP INDEX IF EXISTS idx_core_blocks_chain_id;

CREATE INDEX IF NOT EXISTS idx_core_blocks_chain_id_height ON core_blocks (chain_id, height);

end;
