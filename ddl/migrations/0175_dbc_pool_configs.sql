BEGIN;

CREATE TABLE IF NOT EXISTS sol_meteora_dbc_configs (
    account TEXT PRIMARY KEY,
    slot BIGINT NOT NULL,
    quote_mint TEXT NOT NULL,
    fee_claimer TEXT NOT NULL,
    leftover_receiver TEXT NOT NULL,
    collect_fee_mode SMALLINT NOT NULL,
    migration_option SMALLINT NOT NULL,
    activation_type SMALLINT,
    token_decimal SMALLINT,
    version SMALLINT,
    token_type SMALLINT,
    quote_token_flag SMALLINT,
    partner_locked_lp_percentage SMALLINT,
    partner_lp_percentage SMALLINT,
    creator_locked_lp_percentage SMALLINT,
    creator_lp_percentage SMALLINT,
    migration_fee_option SMALLINT,
    fixed_token_supply_flag SMALLINT,
    creator_trading_fee_percentage SMALLINT,
    token_update_authority SMALLINT,
    migration_fee_percentage SMALLINT,
    creator_migration_fee_percentage SMALLINT,
    swap_base_amount BIGINT,
    migration_quote_threshold BIGINT,
    migration_base_threshold BIGINT,
    migration_sqrt_price NUMERIC,
    pre_migration_token_supply BIGINT,
    post_migration_token_supply BIGINT,
    migrated_collect_fee_mode SMALLINT,
    migrated_dynamic_fee SMALLINT,
    migrated_pool_fee_bps SMALLINT,
    sqrt_start_price NUMERIC,
    curve JSONB,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS sol_meteora_dbc_config_fees (
    config TEXT PRIMARY KEY REFERENCES sol_meteora_dbc_configs(account) ON DELETE CASCADE,
    slot BIGINT NOT NULL,
    base_fee_cliff_fee_numerator BIGINT,
    base_fee_period_frequency BIGINT,
    base_fee_reduction_factor BIGINT,
    base_fee_number_of_period SMALLINT,
    base_fee_fee_scheduler_mode SMALLINT,
    dynamic_fee_initialized SMALLINT,
    dynamic_fee_max_volatility_accumulator INTEGER,
    dynamic_fee_variable_fee_control INTEGER,
    dynamic_fee_bin_step SMALLINT,
    dynamic_fee_filter_period SMALLINT,
    dynamic_fee_decay_period SMALLINT,
    dynamic_fee_reduction_factor SMALLINT,
    dynamic_fee_bin_step_u128 NUMERIC,
    protocol_fee_percent SMALLINT,
    referral_fee_percent SMALLINT
);

CREATE TABLE IF NOT EXISTS sol_meteora_dbc_config_vestings (
    config TEXT PRIMARY KEY REFERENCES sol_meteora_dbc_configs(account) ON DELETE CASCADE,
    slot BIGINT NOT NULL,
    amount_per_period BIGINT,
    cliff_duration_from_migration_time BIGINT,
    frequency BIGINT,
    number_of_period BIGINT,
    cliff_unlock_amount BIGINT
);

COMMIT;