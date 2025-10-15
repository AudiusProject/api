BEGIN;

-- Disable triggers before making bulk updates
ALTER TABLE tracks DISABLE TRIGGER on_track;
ALTER TABLE tracks DISABLE TRIGGER trg_tracks;

-- Update tracks with nft_collection in stream_conditions:
-- 1. Mark them as unlisted (hidden)
-- 2. Drop all stream_conditions
-- 3. Mark as not stream gated
UPDATE tracks
SET 
  is_unlisted = true,
  stream_conditions = null,
  is_stream_gated = false
WHERE is_current = true
  AND is_delete = false
  AND stream_conditions ? 'nft_collection';

-- Re-enable triggers
ALTER TABLE tracks ENABLE TRIGGER on_track;
ALTER TABLE tracks ENABLE TRIGGER trg_tracks;

COMMIT;

