-- Btree index on user_events(blocknumber) — required by the dirty-set
-- scan that drives the Phase 3 (m/r/rv/rd) incremental processors.
--
-- Each tick the processors run:
--   SELECT user_id, blocknumber FROM user_events
--    WHERE blocknumber > $1
--      AND is_current = true
--      AND <type-specific filter>
--    ORDER BY blocknumber ASC LIMIT $2
--
-- Without this index that query is a seq-scan of 2.3M rows (the table only
-- carries a pkey and a user_id btree today). With it the scan is a small
-- index range read keyed on the per-processor checkpoint — same pattern
-- #875 established for follows/saves/reposts/tracks/playlists/users.
--
-- NOTE: intentionally NOT wrapped in BEGIN/COMMIT so CREATE INDEX
-- CONCURRENTLY can run without holding an ACCESS EXCLUSIVE lock on
-- user_events. IF NOT EXISTS makes the migration idempotent.

CREATE INDEX CONCURRENTLY IF NOT EXISTS ix_user_events_blocknumber
    ON public.user_events USING btree (blocknumber);

COMMENT ON INDEX ix_user_events_blocknumber IS
    'Range scans by blocknumber for the incremental Phase 3 challenge processors (m/r/rv/rd).';
