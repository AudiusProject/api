-- idx_playlist_trending_scores_filtered orders (type, version, time_range,
-- playlist_id, score DESC), so it does not match playlist trending callers
-- that filter by type/version/time_range and order by score DESC,
-- playlist_id DESC. idx_playlist_trending_scores_ordered covers that hot
-- order, and playlist_trending_scores_pkey / ix_playlist_trending_scores_playlist_id
-- still cover point lookups by playlist_id.

drop index concurrently if exists public.idx_playlist_trending_scores_filtered;
