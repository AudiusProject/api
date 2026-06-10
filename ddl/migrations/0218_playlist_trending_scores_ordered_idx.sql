-- Hot playlist trending callers order by score DESC, playlist_id DESC after
-- filtering on type/version/time_range. Put the ordering columns immediately
-- after the equality predicates so PostgreSQL can satisfy the LIMIT without a
-- parallel scan and sort.

create index concurrently if not exists idx_playlist_trending_scores_ordered
    on playlist_trending_scores (type, version, time_range, score desc, playlist_id desc);

comment on index idx_playlist_trending_scores_ordered is
    'Covers playlist trending endpoints ordered by score desc, playlist_id desc.';
