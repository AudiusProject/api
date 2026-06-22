-- Hot trending reads now order by score DESC, track_id DESC and are covered by
-- ix_trending_scores_desc_tiebreak / idx_track_trending_scores_for_you_desc_tiebreak.
-- Drop the older ascending-tiebreak indexes to reduce TrendingJob write
-- amplification when it refreshes track_trending_scores.
drop index concurrently if exists public.ix_trending_scores;
drop index concurrently if exists public.idx_track_trending_scores_for_you;
