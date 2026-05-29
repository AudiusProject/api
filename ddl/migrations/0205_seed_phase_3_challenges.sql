-- Phase 3 challenge catalog: mobile-install ("m") and referral
-- ("r"/"rv"/"rd") rewards. These are recomputed by the poll-based
-- processors in api/jobs/challenges/ from the user_events table, which the
-- indexer populates from the `events` object in user-profile metadata
-- (is_mobile_user, referrer) — see indexer/user_events_hook.go. No extra
-- signal table is needed: the source of truth is on-chain profile metadata,
-- same as the Phase 1/2 challenges.

BEGIN;

INSERT INTO challenges (id, type, amount, active, step_count, starting_block, weekly_pool, cooldown_days) VALUES
  ('m',  'boolean',   '1', true,  NULL,        25346436, 25000,      7),
  ('r',  'aggregate', '1', true,  5,           25346436, 25000,      7),
  ('rv', 'aggregate', '1', true,  5000,        25346436, 25000,      7),
  ('rd', 'boolean',   '1', true,  NULL,        25346436, 25000,      7)
ON CONFLICT (id) DO UPDATE SET
  type           = EXCLUDED.type,
  amount         = EXCLUDED.amount,
  active         = EXCLUDED.active,
  step_count     = EXCLUDED.step_count,
  starting_block = EXCLUDED.starting_block,
  weekly_pool    = EXCLUDED.weekly_pool,
  cooldown_days  = EXCLUDED.cooldown_days;

COMMIT;
