ALTER TABLE IF EXISTS artist_coins
ADD COLUMN IF NOT EXISTS dbc_pool TEXT,
ADD COLUMN IF NOT EXISTS damm_v2_pool TEXT;
COMMENT ON COLUMN artist_coins.dbc_pool IS 'The associated DBC pool address for this artist coin, if any. Used in solana indexer.';
COMMENT ON COLUMN artist_coins.damm_v2_pool IS 'The canonical DAMM V2 pool address for this artist coin, if any. Used in solana indexer.';