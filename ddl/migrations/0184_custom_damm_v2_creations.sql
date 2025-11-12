BEGIN;

CREATE TABLE IF NOT EXISTS sol_meteora_damm_v2_initialize_custom_pool_instructions (
    signature TEXT,
    instruction_index INT NOT NULL,
    slot BIGINT NOT NULL,

    creator TEXT,
    position_nft_mint TEXT,
    position_nft_account TEXT,
    payer TEXT,
    pool_authority TEXT,
    pool TEXT,
    position TEXT,
    token_a_mint TEXT,
    token_b_mint TEXT,
    token_a_vault TEXT,
    token_b_vault TEXT,
    payer_token_a TEXT,
    payer_token_b TEXT,
    token_a_program TEXT,
    token_b_program TEXT,
    token_2022_program TEXT,
    system_program TEXT,
    event_authority TEXT,
    program TEXT,
    remaining_accounts JSONB DEFAULT '[]'::jsonb,

    base_fee_cliff_fee_numerator BIGINT,
    base_fee_first_factor INT,
    base_fee_second_factor_max_limiter_duration INT,
    base_fee_second_factor_max_fee_bps INT,
    base_fee_third_factor BIGINT,
    base_fee_mode SMALLINT,

    dynamic_fee_bin_step SMALLINT,
    dynamic_fee_bin_step_u128 NUMERIC,
    dynamic_fee_filter_period SMALLINT,
    dynamic_fee_decay_period SMALLINT,
    dynamic_fee_reduction_factor SMALLINT,
    dynamic_fee_max_volatility_accumulator INT,
    dynamic_fee_variable_fee_control INT,

    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),

    PRIMARY KEY (signature, instruction_index)
);

CREATE INDEX IF NOT EXISTS sol_meteora_damm_v2_initialize_custom_pool_instructions_token_a_mint_idx ON sol_meteora_damm_v2_initialize_custom_pool_instructions(token_a_mint);
COMMENT ON TABLE sol_meteora_damm_v2_initialize_custom_pool_instructions IS 'Tracks InitializeCustomPool instructions for DAMM V2 pools.';
COMMENT ON INDEX sol_meteora_damm_v2_initialize_custom_pool_instructions_token_a_mint_idx IS 'Used for finding artist positions by token_a_mint.';

COMMIT;