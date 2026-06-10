-- One-off correction for the 2026-06-05 UNDERGROUND_TRACKS trending week.
--
-- Context: before the fix in PR #915, the weekly underground winners were
-- computed by reading raw UNDERGROUND_TRACKS scores (which index_trending
-- populates identically to TRACKS, with no eligibility filter), so the
-- winners came out ~identical to regular trending — mainstream artists like
-- BONNIEXCLYDE/Gramatik instead of genuinely niche artists. PR #915 is now
-- deployed, so future weeks are correct; this script repairs 2026-06-05.
--
-- What this does, in ONE transaction:
--   1. Re-derives the CORRECT underground winners LIVE from track_trending_scores
--      using the exact merged-fix query (follower<1500, following<1500, excluding
--      the global top-20 TRACKS). Deriving live (vs. hardcoding ids) guarantees
--      the rows match the deployed algorithm even if scores shifted slightly.
--   2. Replaces the 10 bad trending_results rows (the public "winners" list).
--   3. Removes the UNDISBURSED wrong `tut` rewards and mints the correct ones.
--      Already-disbursed slots (ranks 6/8/9 — diskadebass/vansnydermusic/binap,
--      100 AUDIO each) are LEFT AS-IS: on-chain payouts can't be clawed back, and
--      the ON CONFLICT DO NOTHING below skips those specifiers automatically, so
--      the correct artists for those 3 ranks are intentionally not credited.
--
-- Ordering matters: trending_results is written BEFORE user_challenges so the
-- handle_trending AFTER-INSERT trigger (which recovers the track id from
-- trending_results by rank/type/week/user_id) emits the correct
-- `trending_underground` notification for each freshly-credited artist.
--
-- Single transaction => no other indexer pod ever observes an empty week, so an
-- old/new pod's 30s tick can't race in bad rows. The week ends up with
-- trending_results present, which trips the processor's idempotency guard and
-- keeps the job from re-running this week.

\set ON_ERROR_STOP on
\set week '2026-06-05'

BEGIN;

-- 1. Correct underground winners (mirrors jobs/challenges/trending.go Reconcile
--    underground branch). amount: ranks 1-5 => 1000, ranks 6-10 => 100.
CREATE TEMP TABLE correct_ug ON COMMIT DROP AS
SELECT rank, track_id, user_id,
       CASE WHEN rank <= 5 THEN 1000 ELSE 100 END AS amount
FROM (
  WITH top_trending AS (
    SELECT s.track_id
    FROM track_trending_scores s
    JOIN tracks t ON t.track_id = s.track_id
      AND t.is_current AND NOT t.is_delete AND NOT t.is_unlisted AND t.is_available
    WHERE s.type = 'TRACKS' AND s.version = 'pnagD' AND s.time_range = 'week'
    ORDER BY s.score DESC, s.track_id DESC
    LIMIT 20
  )
  SELECT
    row_number() OVER (ORDER BY s.score DESC, s.track_id DESC)::int AS rank,
    s.track_id,
    t.owner_id AS user_id
  FROM track_trending_scores s
  JOIN tracks t ON t.track_id = s.track_id
    AND t.is_current AND NOT t.is_delete AND NOT t.is_unlisted AND t.is_available
  JOIN aggregate_user au ON au.user_id = t.owner_id
  WHERE s.type = 'TRACKS' AND s.version = 'pnagD' AND s.time_range = 'week'
    AND au.follower_count < 1500
    AND au.following_count < 1500
    AND NOT EXISTS (SELECT 1 FROM top_trending tt WHERE tt.track_id = s.track_id)
) ranked
WHERE rank <= 10;

-- Safety: bail unless we derived a full top-10.
DO $$
BEGIN
  IF (SELECT count(*) FROM correct_ug) <> 10 THEN
    RAISE EXCEPTION 'expected 10 correct underground winners, got %',
      (SELECT count(*) FROM correct_ug);
  END IF;
END $$;

-- 2. Public winners list: replace the 10 bad rows with the correct 10.
DELETE FROM trending_results
WHERE type = 'UNDERGROUND_TRACKS' AND version = 'pnagD' AND week = :'week'::date;

INSERT INTO trending_results (user_id, id, rank, type, version, week)
SELECT user_id, track_id::text, rank, 'UNDERGROUND_TRACKS', 'pnagD', :'week'::date
FROM correct_ug;

-- 3a. Neutralize the wrong, still-unclaimed rewards. The NOT EXISTS guard means
--     we only ever delete specifiers with no on-chain disbursement, so an
--     already-paid slot is never removed even if it was claimed since analysis.
DELETE FROM user_challenges uc
WHERE uc.challenge_id = 'tut'
  AND uc.specifier LIKE :'week' || ':%'
  AND NOT EXISTS (
    SELECT 1 FROM sol_reward_disbursements srd
    WHERE srd.challenge_id = uc.challenge_id
      AND srd.specifier = uc.specifier
  );

-- 3b. Mint the correct rewards. ON CONFLICT DO NOTHING skips any specifier still
--     present (the disbursed ranks 6/8/9), leaving those paid rows untouched.
--     Each inserted row fires handle_trending -> trending_underground notif.
INSERT INTO user_challenges
  (challenge_id, user_id, specifier, is_complete, current_step_count, amount, created_at, completed_at)
SELECT 'tut', user_id, :'week' || ':' || rank, true, 1, amount, now(), now()
FROM correct_ug
ON CONFLICT (challenge_id, specifier) DO NOTHING;

COMMIT;

-- ---------------------------------------------------------------------------
-- Post-correction verification (read-only; runs after COMMIT).
-- ---------------------------------------------------------------------------
\echo
\echo === trending_results after fix (should be the 10 niche winners) ===
SELECT tr.rank, tr.id AS track_id, tr.user_id, u.handle,
       au.follower_count AS followers, au.following_count AS following
FROM trending_results tr
LEFT JOIN users u ON u.user_id = tr.user_id AND u.is_current
LEFT JOIN aggregate_user au ON au.user_id = tr.user_id
WHERE tr.type = 'UNDERGROUND_TRACKS' AND tr.version = 'pnagD' AND tr.week = :'week'::date
ORDER BY tr.rank;

\echo
\echo === tut rewards after fix (ranks 6/8/9 remain the disbursed wrong artists) ===
SELECT uc.specifier, uc.user_id, u.handle, uc.amount,
       (srd.specifier IS NOT NULL) AS disbursed_onchain
FROM user_challenges uc
LEFT JOIN users u ON u.user_id = uc.user_id AND u.is_current
LEFT JOIN sol_reward_disbursements srd
  ON srd.challenge_id = uc.challenge_id AND srd.specifier = uc.specifier
WHERE uc.challenge_id = 'tut' AND uc.specifier LIKE :'week' || ':%'
ORDER BY (split_part(uc.specifier, ':', 2))::int;
