BEGIN;

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

-- Primary key already covers (user_id, timestamp, mint), this index optimizes time-range queries
-- Uses ASC to match ORDER BY timestamp ASC in queries
CREATE INDEX IF NOT EXISTS user_balance_history_user_timestamp_idx 
    ON user_balance_history (user_id, timestamp);

COMMENT ON INDEX user_balance_history_user_timestamp_idx IS 
    'Optimizes time-range queries for a user, ordered by timestamp ASC';

-- Index for finding recent balances
CREATE INDEX IF NOT EXISTS user_balance_history_timestamp_idx 
    ON user_balance_history (timestamp DESC);

COMMENT ON INDEX user_balance_history_timestamp_idx IS 
    'Optimizes queries finding recent balances across all users';

-- Index for future per-mint queries (e.g., "show USDC balance history")
-- This would optimize queries filtering by specific mint(s) and time range
CREATE INDEX IF NOT EXISTS user_balance_history_user_mint_timestamp_idx 
    ON user_balance_history (user_id, mint, timestamp);

COMMENT ON INDEX user_balance_history_user_mint_timestamp_idx IS 
    'Optimizes queries filtering by specific mint(s) and time range (e.g., "show USDC balance history")';

COMMIT;

