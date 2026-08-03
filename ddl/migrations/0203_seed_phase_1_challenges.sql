-- Seed the challenges catalog rows for Phase 1 challenge processors.
--
-- Values mirror apps/packages/discovery-provider/src/challenges/challenges.json
-- as of this writing. Production may already have these rows (from earlier
-- migrations or seeded another way); we use ON CONFLICT DO UPDATE so the
-- catalog stays aligned with the JSON source-of-truth — matching apps'
-- create_new_challenges.py behavior.
--
-- Phase 1 set:
--   p   profile completion           (numeric, 7 steps)
--   u   track upload                 (numeric, 3 tracks)
--   fp  first playlist               (boolean)
--   v   connect-verified             (boolean)
--   e   listen streak                (aggregate; currently inactive)
--   p1/p2/p3  play count milestones  (numeric, 250/1k/10k)
--   tt/tut/tp trending track/under/playlist (trending)

BEGIN;

INSERT INTO challenges (id, type, amount, active, step_count, starting_block, weekly_pool, cooldown_days) VALUES
  ('p',   'numeric',   '1',    true,  7,           0,        25000,      7),
  ('u',   'numeric',   '1',    true,  3,           25346436, 25000,      7),
  ('fp',  'boolean',   '2',    true,  NULL,        28350000, 25000,      7),
  ('v',   'boolean',   '5',    true,  NULL,        0,        25000,      NULL),
  ('e',   'aggregate', '1',    false, 2147483647,  116023891,25000,      NULL),
  ('p1',  'numeric',   '25',   true,  250,         0,        2147483647, 7),
  ('p2',  'numeric',   '100',  true,  1000,        0,        2147483647, 7),
  ('p3',  'numeric',   '1000', true,  10000,       0,        2147483647, 7),
  ('tt',  'trending',  '1000', true,  NULL,        25346436, 100000,     NULL),
  ('tut', 'trending',  '1000', true,  NULL,        25346436, 100000,     NULL),
  ('tp',  'trending',  '100',  true,  NULL,        25346436, 10000,      NULL)
ON CONFLICT (id) DO UPDATE SET
  type           = EXCLUDED.type,
  amount         = EXCLUDED.amount,
  active         = EXCLUDED.active,
  step_count     = EXCLUDED.step_count,
  starting_block = EXCLUDED.starting_block,
  weekly_pool    = EXCLUDED.weekly_pool,
  cooldown_days  = EXCLUDED.cooldown_days;

COMMIT;
