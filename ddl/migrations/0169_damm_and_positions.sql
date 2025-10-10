CREATE TABLE IF NOT EXISTS sol_meteora_dbc_migrations (
    signature TEXT NOT NULL,
    instruction_index INT NOT NULL,
    slot BIGINT NOT NULL,
    dbc_pool TEXT NOT NULL,
    migration_metadata TEXT NOT NULL,
    config TEXT NOT NULL,
    dbc_pool_authority TEXT NOT NULL,
    damm_v2_pool TEXT NOT NULL,
    first_position_nft_mint TEXT NOT NULL,
    first_position_nft_account TEXT NOT NULL,
    first_position TEXT NOT NULL,
    second_position_nft_mint TEXT NOT NULL,
    second_position_nft_account TEXT NOT NULL,
    second_position TEXT NOT NULL,
    damm_pool_authority TEXT NOT NULL,
    base_mint TEXT NOT NULL,
    quote_mint TEXT NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (signature, instruction_index)
);
CREATE INDEX IF NOT EXISTS sol_meteora_dbc_migrations_base_mint_idx ON sol_meteora_dbc_migrations(base_mint);
COMMENT ON TABLE sol_meteora_dbc_migrations IS 'Tracks migrations from DBC pools to DAMM V2 pools.';
COMMENT ON INDEX sol_meteora_dbc_migrations_base_mint_idx IS 'Used for finding artist positions by base_mint.';

CREATE TABLE IF NOT EXISTS sol_meteora_damm_v2_pools (
    address TEXT PRIMARY KEY,
    token_a_mint TEXT NOT NULL,
    token_b_mint TEXT NOT NULL,
    token_a_vault TEXT NOT NULL,
    token_b_vault TEXT NOT NULL,
    whitelisted_vault TEXT NOT NULL,
    partner TEXT NOT NULL,
    liquidity NUMERIC NOT NULL,
    protocol_a_fee BIGINT NOT NULL,
    protocol_b_fee BIGINT NOT NULL,
    partner_a_fee BIGINT NOT NULL,
    partner_b_fee BIGINT NOT NULL,
    sqrt_min_price NUMERIC NOT NULL,
    sqrt_max_price NUMERIC NOT NULL,
    sqrt_price NUMERIC NOT NULL,
    activation_point BIGINT NOT NULL,
    activation_type SMALLINT NOT NULL,
    pool_status SMALLINT NOT NULL,
    token_a_flag SMALLINT NOT NULL,
    token_b_flag SMALLINT NOT NULL,
    collect_fee_mode SMALLINT NOT NULL,
    pool_type SMALLINT NOT NULL,
    fee_a_per_liquidity BIGINT NOT NULL,
    fee_b_per_liquidity BIGINT NOT NULL,
    permanent_lock_liquidity NUMERIC NOT NULL,
    creator TEXT NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
COMMENT ON TABLE sol_meteora_damm_v2_pools IS 'Tracks DAMM V2 pool state. Join with sol_meteora_damm_v2_pool_metrics, sol_meteora_damm_v2_pool_fees, sol_meteora_damm_v2_pool_base_fees, and sol_meteora_damm_v2_pool_dynamic_fees for full pool state.';

CREATE TABLE IF NOT EXISTS sol_meteora_damm_v2_pool_metrics (
    pool TEXT PRIMARY KEY REFERENCES sol_meteora_damm_v2_pools(address) ON DELETE CASCADE,
    total_lp_a_fee NUMERIC NOT NULL,
    total_lp_b_fee NUMERIC NOT NULL,
    total_protocol_a_fee NUMERIC NOT NULL,
    total_protocol_b_fee NUMERIC NOT NULL,
    total_partner_a_fee NUMERIC NOT NULL,
    total_partner_b_fee NUMERIC NOT NULL,
    total_position BIGINT NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP    
);
COMMENT ON TABLE sol_meteora_damm_v2_pool_metrics IS 'Tracks aggregated metrics for DAMM V2 pools. A slice of the DAMM V2 pool state.';

CREATE TABLE IF NOT EXISTS sol_meteora_damm_v2_pool_fees (
    pool TEXT PRIMARY KEY REFERENCES sol_meteora_damm_v2_pools(address) ON DELETE CASCADE,
    protocol_fee_percent SMALLINT NOT NULL,
    partner_fee_percent SMALLINT NOT NULL,
    referral_fee_percent SMALLINT NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
COMMENT ON TABLE sol_meteora_damm_v2_pool_fees IS 'Tracks fee configuration for DAMM V2 pools. A slice of the DAMM V2 pool state.';

CREATE TABLE IF NOT EXISTS sol_meteora_damm_v2_pool_base_fees (
    pool TEXT PRIMARY KEY REFERENCES sol_meteora_damm_v2_pools(address) ON DELETE CASCADE,
    cliff_fee_numerator BIGINT NOT NULL,
    fee_scheduler_mode SMALLINT NOT NULL,
    number_of_period SMALLINT NOT NULL,
    period_frequency BIGINT NOT NULL,
    reduction_factor BIGINT NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
COMMENT ON TABLE sol_meteora_damm_v2_pool_base_fees IS 'Tracks base fee configuration for DAMM V2 pools. A slice of the DAMM V2 pool state.';

CREATE TABLE IF NOT EXISTS sol_meteora_damm_v2_pool_dynamic_fees (
    pool TEXT PRIMARY KEY REFERENCES sol_meteora_damm_v2_pools(address) ON DELETE CASCADE,
    initialized SMALLINT NOT NULL,
    max_volatility_accumulator INTEGER NOT NULL,
    variable_fee_control INTEGER NOT NULL,
    bin_step SMALLINT NOT NULL,
    filter_period SMALLINT NOT NULL,
    decay_period SMALLINT NOT NULL,
    reduction_factor SMALLINT NOT NULL,
    last_update_timestamp BIGINT NOT NULL,
    bin_step_u128 NUMERIC NOT NULL,
    sqrt_price_reference NUMERIC NOT NULL,
    volatility_accumulator NUMERIC NOT NULL,
    volatility_reference NUMERIC NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
COMMENT ON TABLE sol_meteora_damm_v2_pool_dynamic_fees IS 'Tracks dynamic fee configuration for DAMM V2 pools. A slice of the DAMM V2 pool state.';

CREATE TABLE IF NOT EXISTS sol_meteora_damm_v2_positions (
    address TEXT PRIMARY KEY,
    pool TEXT NOT NULL REFERENCES sol_meteora_damm_v2_pools(address) ON DELETE CASCADE,
    nft_mint TEXT NOT NULL,
    fee_a_per_token_checkpoint BIGINT NOT NULL,
    fee_b_per_token_checkpoint BIGINT NOT NULL,
    fee_a_pending BIGINT NOT NULL,
    fee_b_pending BIGINT NOT NULL,
    unlocked_liquidity NUMERIC NOT NULL,
    vested_liquidity NUMERIC NOT NULL,
    permanent_locked_liquidity NUMERIC NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
COMMENT ON TABLE sol_meteora_damm_v2_positions IS 'Tracks DAMM V2 positions representing a claim to the liquidity and associated fees in a DAMM V2 pool. Join with sol_meteora_damm_v2_position_metrics for full position state.';

CREATE TABLE IF NOT EXISTS sol_meteora_damm_v2_position_metrics (
    position TEXT PRIMARY KEY REFERENCES sol_meteora_damm_v2_positions(address) ON DELETE CASCADE,
    total_claimed_a_fee BIGINT NOT NULL,
    total_claimed_b_fee BIGINT NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
COMMENT ON TABLE sol_meteora_damm_v2_position_metrics IS 'Tracks aggregated metrics for DAMM V2 positions. A slice of the DAMM V2 position state.';