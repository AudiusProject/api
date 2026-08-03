-- Partial index covering only public, undeleted, current album playlists.
--
-- Speeds up the album_backlink subquery in GetTracks: that subquery does
-- ~200 random `playlists_pkey` lookups per popular track to filter for
-- (is_album=true AND is_delete=false AND is_current=true) — and ~99.98%
-- of those lookups end up rejected by the filter. With this partial index
-- the planner can resolve "is this an album playlist?" without ever
-- touching the playlists heap for non-album rows.
--
-- Size budget: ~55k matching rows × ~12 bytes ≈ 700 KB.
--
-- NOTE: intentionally NOT wrapped in BEGIN/COMMIT so that
-- CREATE INDEX CONCURRENTLY can run without holding an ACCESS EXCLUSIVE
-- lock on playlists. IF NOT EXISTS makes the migration idempotent.

create index concurrently if not exists idx_playlists_albums_published
    on playlists (playlist_id)
    where is_album = true and is_delete = false and is_current = true;

comment on index idx_playlists_albums_published is
    'Partial index for GetTracks album_backlink subquery; lets non-album lookups skip the heap entirely.';
