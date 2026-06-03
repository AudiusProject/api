BEGIN;

-- API keys are Ethereum-style app addresses. Keep them canonical lowercase so
-- lookups can use the existing api_keys primary key instead of LOWER(api_key).
--
-- Production currently has case-variant duplicate groups, but those groups have
-- identical api_secret/rps/rpm values. Refuse to canonicalize if another
-- environment has conflicting duplicates that would need a product decision.
DO $$
BEGIN
    IF EXISTS (
        SELECT 1
        FROM api_keys
        GROUP BY lower(api_key)
        HAVING count(DISTINCT jsonb_build_array(api_secret, rps, rpm)) > 1
    ) THEN
        RAISE EXCEPTION 'Cannot canonicalize api_keys: lower(api_key) duplicates have conflicting api_secret/rps/rpm values';
    END IF;
END $$;

WITH ranked AS (
    SELECT
        ctid,
        row_number() OVER (
            PARTITION BY lower(api_key)
            ORDER BY (api_key = lower(api_key)) DESC, created_at DESC, api_key ASC
        ) AS row_number
    FROM api_keys
)
DELETE FROM api_keys ak
USING ranked r
WHERE ak.ctid = r.ctid
  AND r.row_number > 1;

WITH ranked AS (
    SELECT
        ctid,
        row_number() OVER (
            PARTITION BY lower(api_key), api_access_key
            ORDER BY is_active DESC, created_at DESC, api_key ASC
        ) AS row_number
    FROM api_access_keys
)
DELETE FROM api_access_keys aak
USING ranked r
WHERE aak.ctid = r.ctid
  AND r.row_number > 1;

UPDATE api_keys
SET api_key = lower(api_key)
WHERE api_key <> lower(api_key);

UPDATE api_access_keys
SET api_key = lower(api_key)
WHERE api_key <> lower(api_key);

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conrelid = 'api_keys'::regclass
          AND conname = 'api_keys_api_key_lowercase_check'
    ) THEN
        ALTER TABLE api_keys
            ADD CONSTRAINT api_keys_api_key_lowercase_check
            CHECK (api_key = lower(api_key)) NOT VALID;
    END IF;
END $$;

ALTER TABLE api_keys
    VALIDATE CONSTRAINT api_keys_api_key_lowercase_check;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conrelid = 'api_access_keys'::regclass
          AND conname = 'api_access_keys_api_key_lowercase_check'
    ) THEN
        ALTER TABLE api_access_keys
            ADD CONSTRAINT api_access_keys_api_key_lowercase_check
            CHECK (api_key = lower(api_key)) NOT VALID;
    END IF;
END $$;

ALTER TABLE api_access_keys
    VALIDATE CONSTRAINT api_access_keys_api_key_lowercase_check;

COMMIT;
