-- Production pg_stat_user_indexes showed both count-only interval play indexes
-- with zero scans. aggregate_interval_plays lookups in the API use track_id,
-- which is covered by interval_play_track_id_idx.

drop index concurrently if exists interval_play_week_count_idx;
drop index concurrently if exists interval_play_month_count_idx;
