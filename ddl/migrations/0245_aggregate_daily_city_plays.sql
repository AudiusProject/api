-- Plays per track per day per listener city, for the Artist Dashboard's
-- top cities (GET /v1/users/:id/listener_cities).
--
-- plays has no index by track, so per-artist location queries can't read it
-- directly. jobs.CityPlaysJob fills this table from plays (checkpointed by
-- plays.id), skipping plays with no city and plays by the track's owner.
-- The first runs backfill whatever plays are still retained (~400 days).

BEGIN;

CREATE TABLE IF NOT EXISTS aggregate_daily_city_plays (
    play_item_id INTEGER NOT NULL,
    day          DATE    NOT NULL,
    country      TEXT    NOT NULL,
    region       TEXT    NOT NULL,
    city         TEXT    NOT NULL,
    count        INTEGER NOT NULL,
    PRIMARY KEY (play_item_id, day, country, region, city)
);

COMMENT ON TABLE aggregate_daily_city_plays IS
    'Plays per track per UTC day per listener city, excluding the track owner''s own plays and plays with no city. Filled by jobs.CityPlaysJob.';

COMMIT;
