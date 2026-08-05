-- Collapse save_type / repost_type 'album' back into 'playlist'.
--
-- An album is a playlist with is_album = true. The indexer briefly derived a
-- separate 'album' type by reading playlists.is_album at index time — a side
-- effect of a fix for entity-id collisions, live from 2026-05-28. It no longer
-- does (OpenAudio/go-openaudio#428). is_album is mutable while save_type is
-- written once, so the same chain history indexed at different times produced
-- different rows. Nothing reads the distinction: every consumer is
-- track / not-track, or ORs the two together.
--
-- THE BINDING CONSTRAINT IS NOT THE PRIMARY KEY. saves_pkey is
-- (user_id, save_item_id, save_type, txhash) and no two rows collide on it. But
-- pkg/etl migration 0030 also added
--
--   saves_current_uniq_idx    ON saves   (user_id, save_item_id, save_type)     WHERE is_current
--   reposts_current_uniq_idx  ON reposts (user_id, repost_item_id, repost_type) WHERE is_current
--
-- which carry no txhash. Two current rows for the same (user, item) are legal
-- today precisely because one is 'album' and one is 'playlist'; collapsing the
-- type makes them duplicates. A blind UPDATE therefore aborts with
--
--   duplicate key value violates unique constraint "saves_current_uniq_idx"
--
-- taking the whole pre-roll migrate Job with it. 43 save pairs and 34 repost
-- pairs collide this way, so the pairs are resolved first and the remainder
-- retyped.
--
-- Winner is the highest blocknumber. Verified against a production clone that
-- this is decidable wherever it matters: of the 43 save pairs, 38 agree on
-- is_delete (either row would do) and 5 disagree — and in every disagreeing
-- pair the blocknumbers differ, with the later row holding the correct state
-- (e.g. user 9014 unfavourited item 613011280 in 2026; the newer row is the
-- delete). Blocknumber ties occur only among pairs that agree, where the choice
-- is immaterial: zero rows both disagree and tie. Reposts are the same shape —
-- 33 agree, 1 disagrees, no tie coincides with a disagreement. The created_at /
-- type tiebreaks below are therefore never load-bearing; they exist only to
-- make the statement deterministic.
--
-- Losers are demoted, not deleted: unlike users these tables do keep superseded
-- history (344 saves, 161 reposts on the clone), so is_current = false is the
-- shape they already use.
--
-- Aggregates: this is a net correction. reconcile_aggregates counts these rows
-- with count(*) and ORs both types, so every colliding pair has been
-- double-counting aggregate_playlist.save_count / repost_count.
--
-- Triggers: on_save / on_repost are suppressed for the whole operation. Their
-- notification group_id embeds the type ('save:<id>:type:<save_type>'), so both
-- the demote and the retype would emit fresh favourite/repost notifications
-- that ON CONFLICT could not dedupe against the existing ':type:album' rows.
-- trg_saves / trg_reposts stay enabled so the search indexer sees the change.
-- Aggregate counts are unaffected by the retype either way: handle_save's delta
-- is transition-aware and evaluates to 0 when is_delete does not change.
--
-- ALTER TABLE ... DISABLE TRIGGER takes ShareRowExclusiveLock — it blocks
-- concurrent writes but not reads, and is held until commit. Blocking writers
-- is the property we want: they wait rather than silently running trigger-free.
-- Each table gets its own transaction so the two locks are never held at once.
--
-- The DO blocks guard on the trigger existing: pg_migrate.sh applies
-- migrations/ before functions/, so on a database bootstrapped from ddl/ alone
-- these triggers do not exist yet.
--
-- Re-running is a no-op once no 'album' rows remain.

BEGIN;
SET LOCAL lock_timeout = '5s';

DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_trigger
               WHERE tgrelid = 'saves'::regclass AND tgname = 'on_save' AND NOT tgisinternal) THEN
        ALTER TABLE saves DISABLE TRIGGER on_save;
    END IF;
END $$;

-- 1. demote the older row of each (user, item) pair holding both types
WITH ranked AS (
    SELECT
        user_id, save_item_id, save_type, txhash,
        row_number() OVER (
            PARTITION BY user_id, save_item_id
            ORDER BY blocknumber DESC NULLS LAST, created_at DESC NULLS LAST, save_type DESC
        ) AS rn
    FROM saves
    WHERE is_current = true
      AND save_type IN ('playlist', 'album')
      AND (user_id, save_item_id) IN (
          SELECT user_id, save_item_id FROM saves
          WHERE is_current = true AND save_type IN ('playlist', 'album')
          GROUP BY user_id, save_item_id
          HAVING count(DISTINCT save_type) > 1
      )
)
UPDATE saves s
SET is_current = false
FROM ranked r
WHERE s.user_id = r.user_id
  AND s.save_item_id = r.save_item_id
  AND s.save_type = r.save_type
  AND s.txhash = r.txhash
  AND r.rn > 1;

-- 2. retype the survivors
UPDATE saves SET save_type = 'playlist' WHERE is_current = true AND save_type = 'album';

DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_trigger
               WHERE tgrelid = 'saves'::regclass AND tgname = 'on_save' AND NOT tgisinternal) THEN
        ALTER TABLE saves ENABLE TRIGGER on_save;
    END IF;
END $$;

COMMIT;

BEGIN;
SET LOCAL lock_timeout = '5s';

DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_trigger
               WHERE tgrelid = 'reposts'::regclass AND tgname = 'on_repost' AND NOT tgisinternal) THEN
        ALTER TABLE reposts DISABLE TRIGGER on_repost;
    END IF;
END $$;

WITH ranked AS (
    SELECT
        user_id, repost_item_id, repost_type, txhash,
        row_number() OVER (
            PARTITION BY user_id, repost_item_id
            ORDER BY blocknumber DESC NULLS LAST, created_at DESC NULLS LAST, repost_type DESC
        ) AS rn
    FROM reposts
    WHERE is_current = true
      AND repost_type IN ('playlist', 'album')
      AND (user_id, repost_item_id) IN (
          SELECT user_id, repost_item_id FROM reposts
          WHERE is_current = true AND repost_type IN ('playlist', 'album')
          GROUP BY user_id, repost_item_id
          HAVING count(DISTINCT repost_type) > 1
      )
)
UPDATE reposts s
SET is_current = false
FROM ranked r
WHERE s.user_id = r.user_id
  AND s.repost_item_id = r.repost_item_id
  AND s.repost_type = r.repost_type
  AND s.txhash = r.txhash
  AND r.rn > 1;

UPDATE reposts SET repost_type = 'playlist' WHERE is_current = true AND repost_type = 'album';

DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_trigger
               WHERE tgrelid = 'reposts'::regclass AND tgname = 'on_repost' AND NOT tgisinternal) THEN
        ALTER TABLE reposts ENABLE TRIGGER on_repost;
    END IF;
END $$;

COMMIT;
