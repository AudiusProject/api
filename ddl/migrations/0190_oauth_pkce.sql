BEGIN;

-- oauth_authorization_codes: short-lived (10 min), one-time-use authorization codes for PKCE flow
CREATE TABLE IF NOT EXISTS oauth_authorization_codes (
    code VARCHAR(255) NOT NULL PRIMARY KEY,
    client_id VARCHAR(255) NOT NULL,
    user_id INTEGER NOT NULL,
    redirect_uri TEXT NOT NULL,
    code_challenge VARCHAR(255) NOT NULL,
    code_challenge_method VARCHAR(10) NOT NULL DEFAULT 'S256',
    scope VARCHAR(50) NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL DEFAULT (NOW() + INTERVAL '10 minutes'),
    used BOOLEAN NOT NULL DEFAULT false
);

CREATE INDEX IF NOT EXISTS idx_oauth_authorization_codes_client_id ON oauth_authorization_codes(client_id);
CREATE INDEX IF NOT EXISTS idx_oauth_authorization_codes_expires_used ON oauth_authorization_codes(expires_at, used);

-- oauth_tokens: opaque access and refresh tokens for PKCE flow
CREATE TABLE IF NOT EXISTS oauth_tokens (
    token VARCHAR(255) NOT NULL PRIMARY KEY,
    token_type VARCHAR(10) NOT NULL,
    client_id VARCHAR(255) NOT NULL,
    user_id INTEGER NOT NULL,
    scope VARCHAR(50) NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    is_revoked BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    refresh_token_id VARCHAR(255),
    family_id VARCHAR(255) NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_oauth_tokens_client_id ON oauth_tokens(client_id);
CREATE INDEX IF NOT EXISTS idx_oauth_tokens_user_id ON oauth_tokens(user_id);
CREATE INDEX IF NOT EXISTS idx_oauth_tokens_family_id ON oauth_tokens(family_id);
CREATE INDEX IF NOT EXISTS idx_oauth_tokens_refresh_token_id ON oauth_tokens(refresh_token_id);
CREATE INDEX IF NOT EXISTS idx_oauth_tokens_lookup ON oauth_tokens(token, token_type, is_revoked, expires_at);

-- oauth_redirect_uris: pre-registered redirect URIs per app
CREATE TABLE IF NOT EXISTS oauth_redirect_uris (
    id SERIAL PRIMARY KEY,
    client_id VARCHAR(255) NOT NULL,
    redirect_uri TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_oauth_redirect_uris_client_id ON oauth_redirect_uris(client_id);

COMMIT;
