BEGIN;

-- Partial index for API-visible tracks (no access_authorities set).
-- All track queries filter with (access_authorities IS NULL OR access_authorities = '{}').
CREATE INDEX IF NOT EXISTS idx_tracks_access_authorities_null
  ON tracks (track_id)
  WHERE (access_authorities IS NULL OR access_authorities = '{}');

COMMIT;
