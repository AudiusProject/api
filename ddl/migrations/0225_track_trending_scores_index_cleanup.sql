-- Keep the current score read paths covered while reducing write amplification
-- for TrendingJob's bulk track_trending_scores refresh. This file is
-- intentionally not wrapped in BEGIN/COMMIT because CREATE/DROP INDEX
-- CONCURRENTLY cannot run inside a transaction block.

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_tts_tracks_pnagd_genre_time_score_desc
    ON track_trending_scores (genre, time_range, score DESC, track_id DESC)
    WHERE type = 'TRACKS'
      AND version = 'pnagD';

COMMENT ON INDEX idx_tts_tracks_pnagd_genre_time_score_desc IS
    'Covers current TRACKS/pnagD genre-filtered trending reads with score desc, track_id desc ordering.';

-- track_trending_scores_pkey starts with track_id and still covers point
-- lookups, so this standalone index is redundant.
DROP INDEX CONCURRENTLY IF EXISTS public.ix_track_trending_scores_track_id;

-- The new partial genre index covers the active TRACKS/pnagD slice with the
-- correct descending tie-break, so the older broad genre indexes only add
-- refresh-time write cost for stale score types/versions.
DROP INDEX CONCURRENTLY IF EXISTS public.ix_track_trending_scores_genre;
DROP INDEX CONCURRENTLY IF EXISTS public.idx_tts_genre_time_score;
