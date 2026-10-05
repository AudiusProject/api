-- Weekly Rotation mixes, one row per listener per period.
--
-- GET /v1/users/:id/weekly-rotation computes a listener's mix on the first
-- request of a period (Wednesday 00:00 UTC, see weeklyrotation.PeriodStart)
-- and stores it here. Later requests that period return the stored list, so
-- the mix and any shared link stay fixed for the week. The next period's mix
-- excludes tracks stored for the previous periods so tracks don't repeat.
--
-- track_ids is ranked, best first. The primary key covers both reads (this
-- period, and the last few periods for one user).

BEGIN;

CREATE TABLE IF NOT EXISTS weekly_rotation_mixes (
    user_id      INTEGER   NOT NULL,
    period_start DATE      NOT NULL,
    track_ids    INTEGER[] NOT NULL,
    created_at   TIMESTAMP WITHOUT TIME ZONE NOT NULL DEFAULT NOW(),
    PRIMARY KEY (user_id, period_start)
);

COMMENT ON TABLE weekly_rotation_mixes IS
    'Ranked Weekly Rotation track ids per listener per period (period_start = Wednesday 00:00 UTC), written on first request of the period.';

COMMIT;
