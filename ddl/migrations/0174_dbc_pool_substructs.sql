BEGIN;

CREATE TABLE IF NOT EXISTS sol_meteora_dbc_pool_metrics (
    pool TEXT PRIMARY KEY,
    slot BIGINT NOT NULL,
    total_protocol_base_fee BIGINT NOT NULL,
    total_protocol_quote_fee BIGINT NOT NULL,
    total_trading_base_fee BIGINT NOT NULL,
    total_trading_quote_fee BIGINT NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS sol_meteora_dbc_pool_volatility_trackers (
    pool TEXT PRIMARY KEY,
    slot BIGINT NOT NULL,
    last_update_timestamp BIGINT NOT NULL,
    volatility_accumulator NUMERIC NOT NULL,
    volatility_reference NUMERIC NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

COMMIT;