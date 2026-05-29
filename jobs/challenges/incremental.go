package challenges

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// Incremental ("dirty-set") processing infrastructure.
//
// The aggregating challenge processors (profile_completion, track_upload,
// first_playlist, cosign) used to rescan their entire source tables on every
// 30s tick. That does not scale: against prod (follows 26M, saves 10M, reposts
// 5.9M, users 3.2M, tracks 1.9M) a full profile_completion recompute timed out
// past 90s and track_upload took ~14s — every cycle.
//
// Instead we checkpoint a monotonic high-water mark per processor and only look
// at rows written since then. Every mutation on these base tables advances
// `blocknumber`: in-place tables (tracks, playlists) bump it on update/delete,
// and append-only versioned tables (follows, saves, reposts, users) insert a
// new row at the current block on every change. So `blocknumber > checkpoint`
// reliably surfaces exactly the rows that changed, and all six source tables
// carry a btree index on blocknumber, so the dirty scan is an index range scan.
//
// Work then becomes proportional to *changes*, not to total table size — which
// is what keeps the fast cadence honest: a profile completed two seconds ago is
// picked up on the very next tick, but we never re-walk 26M follows to find it.
//
// First-run / backfill policy: a fresh checkpoint defaults to 0, which would
// backfill the whole table from block 0. We do NOT want that in prod (the
// legacy Python stack already populated user_challenges, and idempotent upserts
// mean re-deriving history is pure redundant write load). Migration
// 0208_seed_challenge_checkpoints seeds these checkpoints to the current max
// blocknumber on deploy so prod starts "from now". Tests don't run migrations,
// so their checkpoints stay at 0 and small fixtures are processed in one batch.
//
// readCheckpointInt / writeCheckpointInt live in listen_streak.go (the first
// checkpoint-based processor); they're shared package-wide.

// dirtyScanBatch caps how many source rows a single tick will pull. In steady
// state the dirty set is a handful of rows so this never binds; it only bounds
// a catch-up tick after the job (not the indexer) was down for a while. Kept
// modest so the recompute that follows stays well inside one transaction.
const dirtyScanBatch = 5000

// highWaterMark returns the checkpoint value to advance to after scanning
// `scanned` rows whose largest blocknumber was `maxBn`, given the previous
// checkpoint `prev`.
//
// When the batch filled (scanned == dirtyScanBatch) there may be more rows
// sharing maxBn that we have not seen, so we only advance to maxBn-1 to avoid
// skipping them; the next tick re-reads maxBn and recompute is idempotent.
// The one exception: if a single blocknumber overflowed the whole batch
// (practically impossible — a block cannot hold that many social events) we
// advance past it anyway so the scan can never wedge.
func highWaterMark(prev, maxBn int64, scanned int) int64 {
	if scanned == 0 {
		return prev
	}
	if scanned >= dirtyScanBatch {
		if maxBn-1 > prev {
			return maxBn - 1
		}
		return maxBn
	}
	return maxBn
}

// reconcileIncrementalUsers drives the checkpoint + dirty-set pattern for
// processors whose unit of work is "a user whose challenge state may have
// changed".
//
// dirtySQL must SELECT exactly two bigint columns — (user_id, blocknumber) —
// take $1 = previous checkpoint and $2 = batch limit, filter on
// `blocknumber > $1`, and order by blocknumber ASC (so partial batches drain
// oldest-first and the high-water rule is sound). recompute receives the
// distinct affected user ids and is responsible for re-deriving and upserting
// their state; it is skipped when nothing changed. On success the checkpoint
// advances so the same rows are never rescanned.
func reconcileIncrementalUsers(
	ctx context.Context,
	tx pgx.Tx,
	checkpointName string,
	dirtySQL string,
	recompute func(ctx context.Context, tx pgx.Tx, userIDs []int64) error,
) error {
	prev, err := readCheckpointInt(ctx, tx, checkpointName)
	if err != nil {
		return fmt.Errorf("read checkpoint %s: %w", checkpointName, err)
	}

	rows, err := tx.Query(ctx, dirtySQL, prev, dirtyScanBatch)
	if err != nil {
		return fmt.Errorf("dirty scan: %w", err)
	}
	seen := make(map[int64]struct{})
	ids := make([]int64, 0)
	scanned := 0
	maxBn := prev
	for rows.Next() {
		var uid, bn int64
		if err := rows.Scan(&uid, &bn); err != nil {
			rows.Close()
			return err
		}
		scanned++
		if bn > maxBn {
			maxBn = bn
		}
		if _, ok := seen[uid]; !ok {
			seen[uid] = struct{}{}
			ids = append(ids, uid)
		}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}
	if scanned == 0 {
		return nil
	}

	if len(ids) > 0 {
		if err := recompute(ctx, tx, ids); err != nil {
			return err
		}
	}

	newMax := highWaterMark(prev, maxBn, scanned)
	if newMax > prev {
		if err := writeCheckpointInt(ctx, tx, checkpointName, newMax); err != nil {
			return fmt.Errorf("save checkpoint %s: %w", checkpointName, err)
		}
	}
	return nil
}
