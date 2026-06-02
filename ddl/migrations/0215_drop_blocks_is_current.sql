-- Drop the legacy chain-tip marker from blocks.
--
-- The core indexer and API use blocks.number / core_indexed_blocks for block
-- progress and confirmation checks. Maintaining blocks.is_current requires a
-- per-entity-manager-block tip flip on a very large table, and the associated
-- boolean indexes have become a disproportionate source of write work.
--
-- Deploy ordering:
--   1. Deploy an ETL version whose resolveBlockNumber no longer reads, writes,
--      or inserts blocks.is_current.
--   2. Run this migration after the new binary is live everywhere.

BEGIN;

DROP INDEX IF EXISTS public.blocks_is_current_idx;
DROP INDEX IF EXISTS public.is_current_blocks_idx;

ALTER TABLE public.blocks
    DROP COLUMN IF EXISTS is_current;

COMMIT;
