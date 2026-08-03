-- Seed the incremental checkpoints for the dirty-set challenge processors.
--
-- profile_completion (p), track_upload (u), first_playlist (fp), and cosign
-- (cs) were rewritten to recompute only the users/owners/actions touched since
-- a per-processor blocknumber checkpoint, instead of rescanning their entire
-- source tables every tick (a full profile_completion recompute timed out past
-- 90s against prod; track_upload took ~14s). See jobs/challenges/incremental.go.
--
-- A fresh checkpoint defaults to 0, which would backfill each table from block
-- 0 the first time the job runs. We don't want that: the legacy Python stack
-- has already populated user_challenges, and the upserts are idempotent, so a
-- full historical re-derivation is pure redundant write load (and, batched, it
-- would take hours). Seed each checkpoint to the current max blocknumber across
-- its sources so the processors start "from now" and only pick up new activity.
--
-- ON CONFLICT DO NOTHING keeps this idempotent and never rewinds a checkpoint
-- the running job has already advanced. The max(blocknumber) probes are
-- index-only against the per-table blocknumber btrees.
--
-- Checkpoint names must match the constants in the processors:
--   profileCheckpoint        = "challenges:p:last_blocknumber"
--   trackUploadCheckpoint    = "challenges:u:last_blocknumber"
--   firstPlaylistCheckpoint  = "challenges:fp:last_blocknumber"
--   cosignCheckpoint         = "challenges:cs:last_blocknumber"

BEGIN;

-- profile_completion: follows, reposts, saves, users
INSERT INTO indexing_checkpoints (tablename, last_checkpoint)
SELECT 'challenges:p:last_blocknumber', GREATEST(
  COALESCE((SELECT max(blocknumber) FROM follows), 0),
  COALESCE((SELECT max(blocknumber) FROM reposts), 0),
  COALESCE((SELECT max(blocknumber) FROM saves),   0),
  COALESCE((SELECT max(blocknumber) FROM users),   0))
ON CONFLICT (tablename) DO NOTHING;

-- track_upload: tracks
INSERT INTO indexing_checkpoints (tablename, last_checkpoint)
SELECT 'challenges:u:last_blocknumber', COALESCE((SELECT max(blocknumber) FROM tracks), 0)
ON CONFLICT (tablename) DO NOTHING;

-- first_playlist: playlists
INSERT INTO indexing_checkpoints (tablename, last_checkpoint)
SELECT 'challenges:fp:last_blocknumber', COALESCE((SELECT max(blocknumber) FROM playlists), 0)
ON CONFLICT (tablename) DO NOTHING;

-- cosign: reposts, saves (the action sources)
INSERT INTO indexing_checkpoints (tablename, last_checkpoint)
SELECT 'challenges:cs:last_blocknumber', GREATEST(
  COALESCE((SELECT max(blocknumber) FROM reposts), 0),
  COALESCE((SELECT max(blocknumber) FROM saves),   0))
ON CONFLICT (tablename) DO NOTHING;

COMMIT;
