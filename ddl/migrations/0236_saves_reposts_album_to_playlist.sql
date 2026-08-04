-- Collapse save_type / repost_type 'album' back into 'playlist'.
--
-- An album is a playlist with is_album = true. The indexer briefly derived a
-- separate 'album' type by reading playlists.is_album at index time; it was
-- introduced as a side effect of a fix for entity-id collisions (the real bug
-- there was falling through to track inference when the chain said "Playlist")
-- and went live on 2026-05-28. The indexer no longer does this. Two problems
-- with the derived value:
--
--   * is_album is mutable, but save_type is written once and is part of the
--     saves / reposts primary key. Replaying the same chain history at a
--     different time therefore produced different rows.
--   * handle_save / handle_repost build the notification group_id from the
--     type ('save:<id>:type:<save_type>'), so the same favourite could notify
--     twice under two different group_ids.
--
-- Nothing reads the distinction: every consumer is track / not-track, or ORs
-- the two together (get_account_playlists, reconcile_aggregates). Callers that
-- need to know read playlists.is_album at query time, which is what the
-- notification triggers already do.
--
-- Affected rows at time of writing: 670 saves, 528 reposts. There are no
-- primary-key collisions — no (user_id, item_id, txhash) has both a 'playlist'
-- and an 'album' row — so this is a straight UPDATE.
--
-- on_save / on_repost are disabled for the backfill so it does not emit a
-- second round of favourite/repost notifications or re-run milestone checks.
-- Aggregate counts are unaffected either way: handle_save's delta is
-- transition-aware and evaluates to 0 when is_delete does not change.
-- trg_saves / trg_reposts stay enabled so the search indexer still sees the
-- rows change.
--
-- Each table gets its own transaction to keep the ACCESS EXCLUSIVE lock taken
-- by ALTER TABLE ... DISABLE TRIGGER as short as possible. Re-running is a
-- no-op once no 'album' rows remain.
--
-- The 'album' label is deliberately left in the savetype / reposttype enums:
-- Postgres cannot drop an enum value without rebuilding the type, and the
-- indexer no longer writes it.

BEGIN;
SET LOCAL lock_timeout = '5s';

ALTER TABLE saves DISABLE TRIGGER on_save;
UPDATE saves SET save_type = 'playlist' WHERE save_type = 'album';
ALTER TABLE saves ENABLE TRIGGER on_save;

COMMIT;

BEGIN;
SET LOCAL lock_timeout = '5s';

ALTER TABLE reposts DISABLE TRIGGER on_repost;
UPDATE reposts SET repost_type = 'playlist' WHERE repost_type = 'album';
ALTER TABLE reposts ENABLE TRIGGER on_repost;

COMMIT;
