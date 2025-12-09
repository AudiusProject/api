BEGIN;

CREATE TABLE IF NOT EXISTS wheel_spin_results (
    id SERIAL PRIMARY KEY,
    wallet VARCHAR NOT NULL,
    signature VARCHAR NOT NULL UNIQUE,
    mint VARCHAR NOT NULL,
    amount BIGINT NOT NULL,
    prize_id VARCHAR NOT NULL,
    prize_name VARCHAR NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

COMMENT ON TABLE wheel_spin_results IS 'Stores wheel spin results where users pay 250 tokens of any coin to spin and win prizes.';
CREATE INDEX IF NOT EXISTS wheel_spin_results_wallet_idx ON wheel_spin_results (wallet);
COMMENT ON INDEX wheel_spin_results_wallet_idx IS 'Used for getting spin results by wallet.';
CREATE INDEX IF NOT EXISTS wheel_spin_results_signature_idx ON wheel_spin_results (signature);
COMMENT ON INDEX wheel_spin_results_signature_idx IS 'Used for checking if a signature has already been used.';
CREATE INDEX IF NOT EXISTS wheel_spin_results_mint_idx ON wheel_spin_results (mint);
COMMENT ON INDEX wheel_spin_results_mint_idx IS 'Used for getting spin results by coin mint.';

COMMIT;

