-- idx_chain_id_height duplicates the core_indexed_blocks primary key index:
-- both are btree indexes on (chain_id, height). Keep the unique primary-key
-- index and remove the redundant non-unique copy to avoid extra write and
-- storage cost on the core indexer state table.

drop index concurrently if exists public.idx_chain_id_height;
