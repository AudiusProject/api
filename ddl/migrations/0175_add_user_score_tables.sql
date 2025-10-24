begin;

CREATE INDEX IF NOT EXISTS ix_plays_user_hour ON plays (user_id, date_trunc('hour', created_at)) WHERE user_id IS NOT NULL;
COMMENT ON INDEX ix_plays_user_hour IS 'Helps compute distinct hourly plays by user id';


CREATE TABLE IF NOT EXISTS user_distinct_play_hours (
  user_id          integer PRIMARY KEY,
  hours_with_play  integer NOT NULL DEFAULT 0,
  updated_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);

COMMENT ON TABLE user_distinct_play_hours IS 'Tracks the number of distinct hours in which a user has listened to a track';

CREATE TABLE IF NOT EXISTS user_distinct_play_tracks (
  user_id          integer PRIMARY KEY,
  track_count      integer NOT NULL DEFAULT 0,
  updated_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);

COMMENT ON TABLE user_distinct_play_tracks IS 'Tracks the number of distinct tracks a user has listened to';


CREATE TABLE IF NOT EXISTS user_score_features (
  user_id          integer PRIMARY KEY,
  challenge_count  integer default 0,
  updated_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);

COMMENT ON TABLE user_score_features IS 'Tracks some features used in user score calculation';
COMMENT ON COLUMN user_score_features.challenge_count IS 'Tracks the number of fast challenges auser has completed';

CREATE INDEX IF NOT EXISTS ix_au_user_follows
  ON aggregate_user (user_id)
  INCLUDE (follower_count, following_count);
COMMENT ON INDEX ix_au_user_follows IS 'Fast lookup for fields use in karma calculation';

COMMIT;