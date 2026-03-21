begin;

CREATE INDEX IF NOT EXISTS idx_grants_grantee_address
ON grants(grantee_address, is_revoked, created_at DESC)
WHERE is_current = true;

commit;
