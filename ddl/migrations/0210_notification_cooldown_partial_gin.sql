-- Partial GIN on notification(user_ids) WHERE type='reward_in_cooldown'.
--
-- The on_user_challenge trigger fires on every is_complete=true write to
-- user_challenges. For challenges with cooldown_days > 0 it does:
--
--   SELECT id FROM notification
--    WHERE type = 'reward_in_cooldown'
--      AND new.user_id = ANY(user_ids)
--      AND timestamp >= (new.completed_at - interval '1 hour')
--    LIMIT 1;
--
-- The full GIN on user_ids matches across all ~23.5M / 8 GB of notification
-- rows, so each trigger call became IO-bound (DataFileRead). Right after
-- #842 (which added Phase 3 m/r/rv/rd — all cooldown_days=7) deployed,
-- pg_stat_activity caught individual user_challenges upserts spending
-- 19s+ per row, wedging the IndexChallengesJob's first reconcile tick.
--
-- A partial GIN restricted to type='reward_in_cooldown' is small (one type
-- out of ~30) and lets the planner go straight to the candidate user-id
-- matches within that slice; the timestamp filter then runs over a handful
-- of rows instead of the whole table. The benefit applies to every
-- cooldown_days>0 challenge, not just Phase 3.
--
-- NOTE: intentionally NOT wrapped in BEGIN/COMMIT so CREATE INDEX
-- CONCURRENTLY can run without holding an ACCESS EXCLUSIVE lock on
-- notification. IF NOT EXISTS makes the migration idempotent.

CREATE INDEX CONCURRENTLY IF NOT EXISTS ix_notification_cooldown_user_ids
    ON public.notification USING gin (user_ids)
    WHERE type = 'reward_in_cooldown';

COMMENT ON INDEX ix_notification_cooldown_user_ids IS
    'Partial GIN for the on_user_challenge trigger''s cooldown-window check; replaces a multi-second IO-bound scan against the full 8GB notification table with a tiny in-subset lookup.';
