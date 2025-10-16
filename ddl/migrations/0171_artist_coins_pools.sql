
BEGIN;

ALTER TABLE IF EXISTS artist_coins
ADD COLUMN IF NOT EXISTS dbc_pool TEXT,
ADD COLUMN IF NOT EXISTS damm_v2_pool TEXT;
COMMENT ON COLUMN artist_coins.dbc_pool IS 'The associated DBC pool address for this artist coin, if any. Used in solana indexer.';
COMMENT ON COLUMN artist_coins.damm_v2_pool IS 'The canonical DAMM V2 pool address for this artist coin, if any. Used in solana indexer.';

CREATE TABLE IF NOT EXISTS sol_meteora_dbc_pools (
    account TEXT PRIMARY KEY,
    slot BIGINT NOT NULL,
    config TEXT NOT NULL,
    creator TEXT NOT NULL,
    base_mint TEXT NOT NULL,
    base_vault TEXT NOT NULL,
    quote_vault TEXT NOT NULL,
    base_reserve BIGINT NOT NULL,
    quote_reserve BIGINT NOT NULL,
    protocol_base_fee BIGINT NOT NULL,
    protocol_quote_fee BIGINT NOT NULL,
    partner_base_fee BIGINT NOT NULL,
    partner_quote_fee BIGINT NOT NULL,
    sqrt_price NUMERIC NOT NULL,
    activation_point BIGINT NOT NULL,
    pool_type SMALLINT NOT NULL,
    is_migrated SMALLINT NOT NULL,
    is_partner_withdraw_surplus SMALLINT NOT NULL,
    is_protocol_withdraw_surplus SMALLINT NOT NULL,
    migration_progress SMALLINT NOT NULL,
    is_withdraw_leftover SMALLINT NOT NULL,
    is_creator_withdraw_surplus SMALLINT NOT NULL,
    migration_fee_withdraw_status SMALLINT NOT NULL,
    finish_curve_timestamp BIGINT NOT NULL,
    creator_base_fee BIGINT NOT NULL,
    creator_quote_fee BIGINT NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

COMMIT;