-- The For You feed orders weekly track candidates by score DESC, track_id DESC.
-- The older partial index uses track_id ascending, leaving PostgreSQL to do an
-- incremental sort after the index scan. Match the API order exactly.

create index concurrently if not exists idx_track_trending_scores_for_you_desc_tiebreak
    on track_trending_scores (score desc, track_id desc)
    where type = 'TRACKS'
      and version = 'pnagD'
      and time_range = 'week'
      and (genre is null or genre = '');

comment on index idx_track_trending_scores_for_you_desc_tiebreak is
    'Partial index matching For You feed weekly track candidate ordering by score desc, track_id desc.';
