-- When the artist turned on "looking for feedback"; NULL when off.
-- The indexer (go-openaudio pkg/etl) writes it from track metadata.
ALTER TABLE tracks ADD COLUMN IF NOT EXISTS feedback_requested_at timestamp without time zone;

-- Serves /v1/tracks/feedback (newest request first). Almost every row is
-- NULL, so the partial index stays tiny.
CREATE INDEX CONCURRENTLY IF NOT EXISTS tracks_feedback_requested_at_idx
    ON public.tracks USING btree (feedback_requested_at DESC, track_id DESC)
    WHERE feedback_requested_at IS NOT NULL AND is_current = true AND is_delete = false;
