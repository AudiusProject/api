BEGIN;

-- Partial index for API-visible tracks (access_authorities not set).
-- We index rows WHERE access_authorities IS NULL so our queries (which filter
-- access_authorities IS NULL) can use it. A partial index on IS NOT NULL would
-- store less but would never be used for those queries.
CREATE INDEX IF NOT EXISTS idx_tracks_access_authorities_null
  ON tracks (track_id)
  WHERE access_authorities IS NULL;

COMMIT;
