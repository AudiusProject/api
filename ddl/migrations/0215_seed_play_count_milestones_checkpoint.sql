-- Seed the incremental checkpoint for the play-count milestone processor.
--
-- The p1/p2/p3 play milestones (250/1k/10k plays, verified artists only) used to
-- run one non-incremental GROUP BY per tier every tick — a Parallel Seq Scan of
-- ~485k tracks producing ~1,867 rows in ~49s, times three. They were merged into
-- a single processor that checkpoints plays.id and recomputes only the verified
-- artists whose tracks were played since the checkpoint. See
-- jobs/challenges/play_count_milestones.go and jobs/challenges/incremental.go.
--
-- A fresh checkpoint defaults to 0, which would walk the entire plays table from
-- the beginning the first time the job runs. We don't want that: the legacy
-- Python stack already populated user_challenges and the upserts are idempotent,
-- so a full historical re-derivation is pure redundant write load. Seed the
-- checkpoint to the current max(plays.id) so the processor starts "from now" and
-- only picks up new plays.
--
-- ON CONFLICT DO NOTHING keeps this idempotent and never rewinds a checkpoint the
-- running job has already advanced. The max(id) probe is index-only against the
-- plays primary key.
--
-- Checkpoint name must match playCountMilestonesCheckpoint in the processor.

INSERT INTO indexing_checkpoints (tablename, last_checkpoint)
SELECT 'challenges:play_count_milestones:last_play_id', COALESCE((SELECT max(id) FROM plays), 0)
ON CONFLICT (tablename) DO NOTHING;
