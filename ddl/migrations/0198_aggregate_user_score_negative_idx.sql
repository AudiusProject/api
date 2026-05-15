-- Partial index covering only shadowbanned users (aggregate_user.score < 0).
--
-- Several handlers — v1_event_comments, v1_events_remix_contests,
-- v1_fan_club_feed, v1_track_comments, v1_track_comment_count — materialize
-- a `low_abuse_score` CTE via:
--
--     SELECT user_id FROM aggregate_user WHERE score < 0
--
-- Without a covering index this is a sequential scan over the entire
-- aggregate_user table (one row per user, in the millions). On cold cache,
-- /v1/events/remix-contests?status=all measured ~22s end-to-end, dominated
-- by that seq scan; on warm cache the same call returns in ~100ms.
--
-- aggregate_user.score < 0 is a very small fraction of users (shadowbanned
-- accounts only), so a partial index is dramatically cheaper than a full
-- btree on `score`.
--
-- Size budget: thousands of matching rows × ~12 bytes ≈ tens of KB.
--
-- NOTE: intentionally NOT wrapped in BEGIN/COMMIT so CREATE INDEX
-- CONCURRENTLY can run without holding ACCESS EXCLUSIVE on aggregate_user.
-- IF NOT EXISTS keeps the migration idempotent.

create index concurrently if not exists idx_aggregate_user_score_negative
    on aggregate_user (user_id)
    where score < 0;

comment on index idx_aggregate_user_score_negative is
    'Partial index for the shadowban-author CTEs (score < 0); avoids a full table scan of aggregate_user.';
