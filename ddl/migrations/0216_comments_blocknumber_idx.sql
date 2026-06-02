-- Btree index on comments(blocknumber) — required by the dirty-set scan that
-- drives the incremental FirstWeeklyComment processor (challenge "c").
--
-- Each tick the processor runs:
--   SELECT user_id, blocknumber FROM comments
--    WHERE blocknumber > $1
--    ORDER BY blocknumber ASC LIMIT $2
--
-- Without this index that query is a seq-scan of the whole comments table
-- (~321k rows) every 30s tick. With it the scan is a small index range read
-- keyed on the processor checkpoint — same pattern #875 established for
-- follows/saves/reposts/tracks/playlists/users and #897 for user_events.
--
-- NOTE: intentionally NOT wrapped in BEGIN/COMMIT so CREATE INDEX
-- CONCURRENTLY can run without holding an ACCESS EXCLUSIVE lock on comments.
-- IF NOT EXISTS makes the migration idempotent.

CREATE INDEX CONCURRENTLY IF NOT EXISTS comments_blocknumber_idx
    ON public.comments USING btree (blocknumber);

COMMENT ON INDEX comments_blocknumber_idx IS
    'Range scans by blocknumber for the incremental FirstWeeklyComment challenge processor (c).';
