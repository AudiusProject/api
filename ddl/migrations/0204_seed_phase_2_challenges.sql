-- Seed catalog rows for Phase 2 challenge processors.
--
-- Values mirror apps/packages/discovery-provider/src/challenges/challenges.json
-- with ON CONFLICT DO UPDATE so the catalog stays aligned (matches apps'
-- create_new_challenges.py behavior).
--
-- Phase 2 set (all aggregate type — they accumulate per-occurrence):
--   c   first_weekly_comment      (each ISO week)
--   t   tastemaker                (10 catalog amount × per-track win)
--   cp  comment_pin               (pinned by verified artist on their track)
--   cs  cosign                    (parent owner cosigns a remix; CURRENTLY INACTIVE in apps)
--   w   remix_contest_winner      (winner of remix contest hosted by verified user)
--   b   audio_matching_buyer      (USDC content purchase, file under buyer)
--   s   audio_matching_seller     (same purchase, file under verified seller)

BEGIN;

INSERT INTO challenges (id, type, amount, active, step_count, starting_block, weekly_pool, cooldown_days) VALUES
  ('c',  'aggregate', '1',    true,  2147483647, 0,         2147483647, 7),
  ('t',  'aggregate', '100',  true,  2147483647, 0,         2147483647, 7),
  ('cp', 'aggregate', '10',   true,  2147483647, 1979515,   2147483647, 7),
  ('cs', 'aggregate', '1000', false, 2147483647, 95017582,  50000,      7),
  ('w',  'aggregate', '1000', true,  2147483647, 98950182,  50000,      7),
  ('b',  'aggregate', '1',    true,  2147483647, 220157041, 25000,      7),
  ('s',  'aggregate', '5',    true,  2147483647, 220157041, 25000,      7)
ON CONFLICT (id) DO UPDATE SET
  type           = EXCLUDED.type,
  amount         = EXCLUDED.amount,
  active         = EXCLUDED.active,
  step_count     = EXCLUDED.step_count,
  starting_block = EXCLUDED.starting_block,
  weekly_pool    = EXCLUDED.weekly_pool,
  cooldown_days  = EXCLUDED.cooldown_days;

COMMIT;
