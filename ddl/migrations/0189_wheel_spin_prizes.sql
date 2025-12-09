BEGIN;

CREATE TABLE IF NOT EXISTS wheel_spin_prizes (
    id SERIAL PRIMARY KEY,
    config_id INT NOT NULL,
    prize_id VARCHAR NOT NULL,
    name VARCHAR NOT NULL,
    description TEXT,
    weight INT NOT NULL DEFAULT 1,
    is_active BOOLEAN NOT NULL DEFAULT true,
    metadata JSONB,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (config_id) REFERENCES wheel_spin_configs(id) ON DELETE CASCADE,
    UNIQUE(config_id, prize_id)
);

COMMENT ON TABLE wheel_spin_prizes IS 'Defines prizes available for each wheel spin configuration. Prizes are selected randomly based on weight.';
CREATE INDEX IF NOT EXISTS wheel_spin_prizes_config_id_idx ON wheel_spin_prizes (config_id);
COMMENT ON INDEX wheel_spin_prizes_config_id_idx IS 'Used for looking up prizes by config.';
CREATE INDEX IF NOT EXISTS wheel_spin_prizes_active_idx ON wheel_spin_prizes (config_id, is_active);
COMMENT ON INDEX wheel_spin_prizes_active_idx IS 'Used for filtering active prizes for a config.';

COMMIT;

