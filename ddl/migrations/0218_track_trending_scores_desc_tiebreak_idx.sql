-- Hot trending endpoints order by score desc, track_id desc. The existing
-- ix_trending_scores index has track_id in ascending order, which leaves
-- PostgreSQL doing an incremental sort for every request. Keep API tie-break
-- behavior stable by adding an index whose order matches the query.
CREATE INDEX CONCURRENTLY IF NOT EXISTS ix_trending_scores_desc_tiebreak
    ON track_trending_scores (type, version, time_range, score DESC, track_id DESC);
