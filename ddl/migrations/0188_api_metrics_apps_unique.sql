BEGIN;

CREATE TABLE IF NOT EXISTS api_metrics_apps_unique (
    date DATE NOT NULL,
    app_name VARCHAR(255) NOT NULL,
    hll_sketch BYTEA,
    total_count BIGINT NOT NULL DEFAULT 0,
    unique_count BIGINT NOT NULL DEFAULT 0,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    PRIMARY KEY (date, app_name)
);

COMMENT ON TABLE api_metrics_apps_unique IS 'Stores HLL sketches for tracking unique users per application. app_name stores the identifier (api_key if present, otherwise app_name from request).';

CREATE INDEX IF NOT EXISTS idx_api_metrics_apps_unique_date ON api_metrics_apps_unique(date);
CREATE INDEX IF NOT EXISTS idx_api_metrics_apps_unique_app_name ON api_metrics_apps_unique(app_name);

COMMIT;

