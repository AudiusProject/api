-- Seed the incremental checkpoints for the Phase 3 user_events processors.
--
-- mobile_install (m), referral (r), verified_referral (rv), and referred
-- (rd) are now rewritten in the same dirty-set style as #875's other
-- aggregating processors: each tick only rescans user_events rows whose
-- blocknumber moved past a per-processor checkpoint. See
-- jobs/challenges/user_event_challenges.go.
--
-- A fresh checkpoint defaults to 0, which would re-derive every completion
-- over ~2.3M user_events rows on first run — exactly the load that wedged
-- the cd94ede (#842) deploy's first reconcile tick: each completed upsert
-- fired the handle_on_user_challenge trigger, whose cooldown_days>0 path
-- scans the 8GB notification table. Seed each checkpoint to the current
-- max(user_events.blocknumber) so prod starts "from now" and skips the
-- redundant backfill. Python already populated user_challenges and the
-- upserts are idempotent.
--
-- ON CONFLICT DO NOTHING keeps this idempotent and never rewinds a
-- checkpoint the running job has already advanced. The max(blocknumber)
-- probe is index-only against ix_user_events_blocknumber (added in 0209).
--
-- Checkpoint names must match the constants in user_event_challenges.go.

BEGIN;

-- mobile_install
INSERT INTO indexing_checkpoints (tablename, last_checkpoint)
SELECT 'challenges:m:last_blocknumber',
       COALESCE((SELECT max(blocknumber) FROM user_events), 0)
ON CONFLICT (tablename) DO NOTHING;

-- referral (unverified tier)
INSERT INTO indexing_checkpoints (tablename, last_checkpoint)
SELECT 'challenges:r:last_blocknumber',
       COALESCE((SELECT max(blocknumber) FROM user_events), 0)
ON CONFLICT (tablename) DO NOTHING;

-- referral (verified tier)
INSERT INTO indexing_checkpoints (tablename, last_checkpoint)
SELECT 'challenges:rv:last_blocknumber',
       COALESCE((SELECT max(blocknumber) FROM user_events), 0)
ON CONFLICT (tablename) DO NOTHING;

-- referred (the referred user's side)
INSERT INTO indexing_checkpoints (tablename, last_checkpoint)
SELECT 'challenges:rd:last_blocknumber',
       COALESCE((SELECT max(blocknumber) FROM user_events), 0)
ON CONFLICT (tablename) DO NOTHING;

COMMIT;
