BEGIN;

-- Table to store historical user balance snapshots per mint
CREATE TABLE IF NOT EXISTS user_balance_history (
    user_id INTEGER NOT NULL,
    mint TEXT NOT NULL,
    timestamp TIMESTAMP NOT NULL,
    balance BIGINT NOT NULL,
    balance_usd DOUBLE PRECISION NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (user_id, timestamp, mint)
);

COMMENT ON TABLE user_balance_history IS 'Stores historical snapshots of user token balances per mint, binned hourly by timestamp';
COMMENT ON COLUMN user_balance_history.user_id IS 'The user ID this balance snapshot belongs to';
COMMENT ON COLUMN user_balance_history.mint IS 'The token mint address';
COMMENT ON COLUMN user_balance_history.timestamp IS 'The binned timestamp (hourly) for this balance snapshot';
COMMENT ON COLUMN user_balance_history.balance IS 'The raw token balance (in token units, accounting for decimals)';
COMMENT ON COLUMN user_balance_history.balance_usd IS 'The USD value of this token balance at this timestamp';
COMMENT ON COLUMN user_balance_history.created_at IS 'When this record was created';

-- Index for efficient queries by user and time range (used for GROUP BY timestamp)
CREATE INDEX IF NOT EXISTS user_balance_history_user_timestamp_idx 
    ON user_balance_history (user_id, timestamp DESC);

-- Index for finding recent balances
CREATE INDEX IF NOT EXISTS user_balance_history_timestamp_idx 
    ON user_balance_history (timestamp DESC);

-- Index for queries filtering by mint
CREATE INDEX IF NOT EXISTS user_balance_history_mint_idx 
    ON user_balance_history (mint);

COMMIT;

