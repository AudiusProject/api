-- Reconcile the playlist reverse index stored on tracks with playlist_tracks.
--
-- tracks.playlists_containing_track is part of album-purchase authorization:
-- when a track is gated, the API checks whether the listener bought an album
-- that currently contains it. The legacy Python indexer maintained this array
-- alongside playlist_tracks, but the Go ETL initially maintained only the
-- junction table. Rows indexed during that gap therefore have an authoritative
-- playlist_tracks relation without the corresponding reverse index.
--
-- This migration treats playlist_tracks as the source of truth and repairs all
-- state that can be established without guessing:
--
--   * active relations are present in playlists_containing_track;
--   * removed relations are absent from playlists_containing_track; and
--   * an active relation has no stale entry in
--     playlists_previously_containing_track.
--
-- Missing historical-removal entries are deliberately not synthesized.
-- playlist_tracks.updated_at is written with now(), not block time, while the
-- API compares the removal timestamp with a purchase timestamp. Using that
-- wall-clock value could grant access to someone who bought the album after
-- the on-chain removal but before a delayed indexer processed it. Existing
-- removal entries are preserved; the ETL version shipped with this migration
-- writes exact block timestamps for future removals.
--
-- Rollout note: ddl migrations run in the pre-roll Job, before the new ETL
-- indexer replaces the old one. The old indexer can therefore create a small
-- number of additional mismatches between this transaction committing and the
-- rollout completing. This migration is intentionally safe to run again after
-- the new indexer owns the writer; the PR rollout notes include that rerun and
-- a zero-mismatch verification query.
--
-- The set comparison ignores array order, so already-correct tracks do not get
-- rewritten merely because their playlist ids were accumulated in a different
-- order. Changed arrays are normalized to ascending playlist id order.
--
-- Updating tracks fires two broad triggers in production. trg_tracks is kept
-- enabled so search/index consumers observe repaired rows. on_track is disabled
-- because it recounts the owner's entire catalog once per updated track; none
-- of the fields changed here affect that aggregate or create notifications.
-- The temporary table records whether this migration disabled the trigger so a
-- pre-disabled trigger is not accidentally enabled at the end. On a database
-- bootstrapped from ddl/ alone, migrations run before functions and on_track
-- does not exist yet, so both trigger operations become no-ops.
--
-- A SHARE lock keeps playlist_tracks stable while the expected sets are built.
-- It blocks playlist membership writes, but not reads, for this transaction.
-- The tracks trigger lock and row updates likewise block competing writes only
-- for the duration of the backfill. Re-running is a no-op once the sets agree.

BEGIN;
SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = 0;

LOCK TABLE playlist_tracks IN SHARE MODE;

CREATE TEMP TABLE track_playlist_backfill_trigger_state
ON COMMIT DROP AS
SELECT 1 AS disabled_by_this_migration
FROM pg_trigger
WHERE tgrelid = 'tracks'::regclass
  AND tgname = 'on_track'
  AND NOT tgisinternal
  AND tgenabled <> 'D';

DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM track_playlist_backfill_trigger_state) THEN
        ALTER TABLE tracks DISABLE TRIGGER on_track;
    END IF;
END $$;

WITH expected AS MATERIALIZED (
    SELECT
        track_id,
        COALESCE(
            array_agg(playlist_id ORDER BY playlist_id)
                FILTER (WHERE is_removed = false),
            '{}'::integer[]
        ) AS active_playlist_ids
    FROM playlist_tracks
    GROUP BY track_id
)
UPDATE tracks t
SET
    playlists_containing_track = e.active_playlist_ids,
    playlists_previously_containing_track =
        CASE
            WHEN cardinality(e.active_playlist_ids) = 0
                THEN t.playlists_previously_containing_track
            ELSE t.playlists_previously_containing_track - ARRAY(
                SELECT playlist_id::text
                FROM unnest(e.active_playlist_ids) AS playlist_id
            )
        END
FROM expected e
WHERE t.track_id = e.track_id
  AND t.is_current = true
  AND (
      NOT (
          t.playlists_containing_track @> e.active_playlist_ids
          AND e.active_playlist_ids @> t.playlists_containing_track
      )
      OR EXISTS (
          SELECT 1
          FROM unnest(e.active_playlist_ids) AS playlist_id
          WHERE jsonb_exists(
              t.playlists_previously_containing_track,
              playlist_id::text
          )
      )
  );

DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM track_playlist_backfill_trigger_state) THEN
        ALTER TABLE tracks ENABLE TRIGGER on_track;
    END IF;
END $$;

COMMIT;
