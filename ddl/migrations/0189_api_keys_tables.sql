BEGIN;

-- api_keys: stores API key credentials with rate limit settings
CREATE TABLE IF NOT EXISTS api_keys (
    api_key VARCHAR(255) NOT NULL PRIMARY KEY,
    api_secret VARCHAR(255),
    rps INTEGER NOT NULL DEFAULT 10,
    rpm INTEGER NOT NULL DEFAULT 500000,
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

-- api_access_keys: stores access keys associated with API keys
CREATE TABLE IF NOT EXISTS api_access_keys (
    api_key VARCHAR(255) NOT NULL,
    api_access_key VARCHAR(255) NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    is_active BOOLEAN NOT NULL DEFAULT true,
    PRIMARY KEY (api_key, api_access_key)
);

CREATE INDEX IF NOT EXISTS idx_api_access_keys_api_access_key ON api_access_keys(api_access_key);
CREATE INDEX IF NOT EXISTS idx_api_access_keys_is_active ON api_access_keys(api_key, is_active) WHERE is_active = true;

-- Migrate existing developer apps into api_keys
INSERT INTO api_keys (api_key, api_secret, created_at)
SELECT DISTINCT ON (da.address)
    da.address,
    NULL,
    da.created_at
FROM developer_apps da
WHERE da.is_current = true
  AND da.is_delete = false
ORDER BY da.address, da.created_at DESC
ON CONFLICT (api_key) DO NOTHING;

COMMIT;
