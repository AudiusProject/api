BEGIN;

-- Create prizes table
CREATE TABLE IF NOT EXISTS prizes (
    id SERIAL PRIMARY KEY,
    prize_id VARCHAR NOT NULL UNIQUE,
    name VARCHAR NOT NULL,
    description TEXT,
    weight INT NOT NULL DEFAULT 1,
    is_active BOOLEAN NOT NULL DEFAULT true,
    metadata JSONB,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

COMMENT ON TABLE prizes IS 'Defines prizes available for wheel spins. Prizes are selected randomly based on weight.';
CREATE INDEX IF NOT EXISTS prizes_active_idx ON prizes (is_active);
COMMENT ON INDEX prizes_active_idx IS 'Used for filtering active prizes.';

-- Create claimed_prizes table
CREATE TABLE IF NOT EXISTS claimed_prizes (
    id SERIAL PRIMARY KEY,
    wallet VARCHAR NOT NULL,
    signature VARCHAR NOT NULL UNIQUE,
    mint VARCHAR NOT NULL,
    amount BIGINT NOT NULL,
    prize_id VARCHAR NOT NULL,
    prize_name VARCHAR NOT NULL,
    prize_type VARCHAR,
    action_data JSONB,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

COMMENT ON TABLE claimed_prizes IS 'Stores claimed prizes where users pay tokens to spin and win prizes.';
CREATE INDEX IF NOT EXISTS claimed_prizes_wallet_idx ON claimed_prizes (wallet);
COMMENT ON INDEX claimed_prizes_wallet_idx IS 'Used for getting claimed prizes by wallet.';
CREATE INDEX IF NOT EXISTS claimed_prizes_signature_idx ON claimed_prizes (signature);
COMMENT ON INDEX claimed_prizes_signature_idx IS 'Used for checking if a signature has already been used.';
CREATE INDEX IF NOT EXISTS claimed_prizes_mint_idx ON claimed_prizes (mint);
COMMENT ON INDEX claimed_prizes_mint_idx IS 'Used for getting claimed prizes by coin mint.';

COMMIT;

