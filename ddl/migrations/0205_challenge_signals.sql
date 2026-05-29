-- challenge_signals: client- and admin-reported events that don't surface
-- from any on-chain or Solana table on their own. Consumed by the m/r/
-- rv/rd challenge processors in api/jobs/challenges/.
--
-- One row per discrete event. Rows are append-only — processors track
-- their own checkpoint into indexing_checkpoints.

BEGIN;

DO $$ BEGIN
  CREATE TYPE challenge_signal_type AS ENUM (
    'mobile_install',
    'referral'
  );
EXCEPTION
  WHEN duplicate_object THEN NULL;
END $$;

CREATE TABLE IF NOT EXISTS challenge_signals (
  id            bigserial PRIMARY KEY,
  type          challenge_signal_type NOT NULL,
  user_id       integer NOT NULL,
  extra         jsonb NOT NULL DEFAULT '{}'::jsonb,
  created_at    timestamptz NOT NULL DEFAULT now(),
  source        varchar,
  client_nonce  varchar
);

-- Dedupe replays of the same client-reported event.
CREATE UNIQUE INDEX IF NOT EXISTS challenge_signals_nonce_idx
  ON challenge_signals (type, user_id, client_nonce)
  WHERE client_nonce IS NOT NULL;

-- For each processor's incremental scan.
CREATE INDEX IF NOT EXISTS challenge_signals_type_id_idx
  ON challenge_signals (type, id);

-- Phase 3 catalog rows.
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
