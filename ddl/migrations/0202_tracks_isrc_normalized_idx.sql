-- Functional index supporting GetTrackIdsByISRC. The handler normalizes the
-- incoming ?isrc= param (uppercase + strip non-alphanumerics) and the SQL
-- compares against regexp_replace(upper(isrc), '[^A-Z0-9]', '', 'g'), so a
-- plain btree on isrc cannot be used. This index materializes the same
-- normalized expression, allowing dash/case-insensitive ISRC lookups to be
-- index-scanned instead of seq-scanning ~all tracks. Partial on
-- isrc IS NOT NULL since the vast majority of rows have no ISRC.
--
-- NOTE: intentionally NOT wrapped in BEGIN/COMMIT so CREATE INDEX
-- CONCURRENTLY can run without taking an ACCESS EXCLUSIVE lock on tracks.
-- IF NOT EXISTS makes the migration idempotent.

create index concurrently if not exists tracks_isrc_normalized_idx
    on public.tracks (
        (regexp_replace(upper(isrc), '[^A-Z0-9]'::text, ''::text, 'g'))
    )
    where isrc is not null;

comment on index public.tracks_isrc_normalized_idx is
    'Functional index supporting GetTrackIdsByISRC; matches regexp_replace(upper(isrc), ''[^A-Z0-9]'', '''', ''g'') so dash/case-insensitive ?isrc= lookups hit the index.';
