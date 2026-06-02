-- Seed the incremental checkpoint for the FirstWeeklyComment processor.
--
-- Challenge "c" used to re-run a DISTINCT (user_id, ISO-year, ISO-week) scan
-- over the entire comments table (~321k rows) and re-upsert a user_challenge
-- row for every (user, week) pair across all history every tick — ~100s, the
-- dominant cost of IndexChallengesJob. It now checkpoints comments.blocknumber
-- and recomputes only the users whose comments changed since the checkpoint.
-- See jobs/challenges/first_weekly_comment.go and jobs/challenges/incremental.go.
--
-- A fresh checkpoint defaults to 0, which would re-derive every (user, week)
-- pair from the beginning the first time the job runs. We don't want that: the
-- legacy Python stack already populated user_challenges and the upserts are
-- idempotent, so a full historical re-derivation is pure redundant write load.
-- Seed the checkpoint to the current max(comments.blocknumber) so the processor
-- starts "from now" and only picks up new comments.
--
-- ON CONFLICT DO NOTHING keeps this idempotent and never rewinds a checkpoint the
-- running job has already advanced. The max(blocknumber) probe is an index-only
-- scan against comments_blocknumber_idx (migration 0216).
--
-- Checkpoint name must match commentCheckpoint in the processor.

INSERT INTO indexing_checkpoints (tablename, last_checkpoint)
SELECT 'challenges:c:last_blocknumber', COALESCE((SELECT max(blocknumber) FROM comments), 0)
ON CONFLICT (tablename) DO NOTHING;
