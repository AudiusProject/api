BEGIN;

ALTER TABLE users ADD COLUMN IF NOT EXISTS last_active_at TIMESTAMPTZ DEFAULT NULL;
COMMENT ON COLUMN users.last_active_at IS 'Timestamp of the user''s most recent app-open event, updated by POST /v1/users/me/ping.';

COMMIT;
