-- The scheduled-release publisher runs once per minute and updates only due
-- scheduled tracks/albums. Without these partial indexes, idle ticks scan tens
-- of thousands of current public/unlisted rows to discover that nothing is due.
--
-- NOTE: intentionally NOT wrapped in BEGIN/COMMIT so CREATE INDEX
-- CONCURRENTLY can run without holding an ACCESS EXCLUSIVE lock on tracks or
-- playlists. IF NOT EXISTS makes the migration idempotent.

create index concurrently if not exists tracks_scheduled_release_due_idx
    on public.tracks (release_date)
    where is_unlisted = true
      and is_scheduled_release = true
      and release_date is not null
      and is_current = true
      and is_delete = false;

comment on index public.tracks_scheduled_release_due_idx is
    'Covers the scheduled-release publisher tracks update by release_date for due unlisted scheduled tracks.';

create index concurrently if not exists playlists_scheduled_release_due_idx
    on public.playlists (release_date)
    where is_private = true
      and is_album = true
      and is_scheduled_release = true
      and release_date is not null
      and is_current = true
      and is_delete = false;

comment on index public.playlists_scheduled_release_due_idx is
    'Covers the scheduled-release publisher album update by release_date for due private scheduled albums.';
