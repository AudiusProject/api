BEGIN;

CREATE TABLE IF NOT EXISTS wheel_spin_configs (
    id SERIAL PRIMARY KEY,
    mint VARCHAR NOT NULL,
    amount BIGINT NOT NULL,
    name VARCHAR,
    description TEXT,
    is_active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(mint, amount)
);

COMMENT ON TABLE wheel_spin_configs IS 'Defines valid wheel spin configurations. Each config specifies a mint and amount combination that is allowed for spins.';
CREATE INDEX IF NOT EXISTS wheel_spin_configs_mint_idx ON wheel_spin_configs (mint);
COMMENT ON INDEX wheel_spin_configs_mint_idx IS 'Used for looking up configs by mint.';
CREATE INDEX IF NOT EXISTS wheel_spin_configs_active_idx ON wheel_spin_configs (is_active);
COMMENT ON INDEX wheel_spin_configs_active_idx IS 'Used for filtering active configs.';

COMMIT;

