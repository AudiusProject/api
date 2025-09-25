begin;

CREATE INDEX IF NOT EXISTS agg_user_has_tracks_idx
  ON aggregate_user (user_id)
  WHERE total_track_count > 0;

commit;
