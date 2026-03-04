BEGIN;

-- access_authorities: wallet addresses that can sign to authorize stream access (programmable distribution)
ALTER TABLE tracks ADD COLUMN IF NOT EXISTS access_authorities TEXT[];

COMMIT;
