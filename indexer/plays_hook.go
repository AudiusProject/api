package indexer

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	etl "github.com/OpenAudio/go-openaudio/pkg/etl"
	"go.uber.org/zap"
)

// newPlaysHook returns a pkg/etl PlaysHook that writes each on-chain play
// into api/'s `plays` table, restoring the behavior of the legacy Python
// discovery-provider task `index_core_plays`.
//
// Background: the vendored ETL play processor only writes its own
// `etl_plays` table; nothing in api/ reads `etl_plays`. The `plays` table is
// the one every downstream consumer depends on — the `on_play` trigger fans
// a row out to aggregate_plays / aggregate_monthly_plays / milestones /
// notifications / user_distinct_play_*; the challenge processors
// (listen_streak, play_count_milestones) poll `plays` directly; trending and
// hourly-play-count jobs read it. This hook bridges that gap.
//
// The hook runs in the same DB transaction (savepoint) the play processor
// used for etl_plays (etl.PlaysParams.DBTX), so the `plays` rows commit
// atomically with etl_plays and the rest of the block.
//
// Field mapping mirrors index_core_plays exactly:
//   - play_item_id = int(track_id); the play is SKIPPED if track_id is not an
//     integer (matches Python's `try: int(track_id) except: continue`).
//   - user_id = int(user_id), or NULL for an anonymous listen when user_id is
//     not an integer (Python's "Recording anonymous listen" path).
//   - source = "relay", created_at = play timestamp, updated_at = now().
//   - city/region/country/signature copied through.
//   - slot = Core block height: a monotonically increasing integer shared by
//     all plays in a block, the same shape as Python's shared per-tx
//     next_slot. (`plays.slot` is read by no api/ Go consumer; the `on_play`
//     trigger only forwards it onto milestone/notification rows.)
//
// Unlike Python, this hook does NOT dispatch challenge events — the new
// challenge processors reconcile from `plays` by polling, so the bridge only
// needs to land the rows.
//
// Hook errors are logged but not propagated (etl.PlaysHook contract): a
// malformed play must not roll back etl_plays or halt the indexer.
func newPlaysHook(logger *zap.Logger) etl.PlaysHook {
	hookLogger := logger.Named("PlaysHook")
	return func(ctx context.Context, params *etl.PlaysParams) error {
		start := time.Now()
		if err := indexPlays(ctx, params); err != nil {
			hookLogger.Warn("failed to index plays into plays table",
				zap.String("tx_hash", params.TxHash),
				zap.Int64("block_height", params.BlockHeight),
				zap.Int("plays", len(params.Plays)),
				zap.Duration("duration", time.Since(start)),
				zap.Error(err))
		} else if elapsed := time.Since(start); elapsed > time.Second {
			hookLogger.Info("indexed plays into plays table",
				zap.String("tx_hash", params.TxHash),
				zap.Int64("block_height", params.BlockHeight),
				zap.Int("plays", len(params.Plays)),
				zap.Duration("duration", elapsed))
		}
		return nil
	}
}

func indexPlays(ctx context.Context, params *etl.PlaysParams) error {
	plays := params.Plays
	if len(plays) == 0 {
		return nil
	}

	// slot is shared by every play in this block (mirrors Python's shared
	// per-tx next_slot). Block height is monotonic and comfortably within
	// int range for the foreseeable life of the chain.
	slot := int(params.BlockHeight)
	now := time.Now()

	// Columns per row: user_id, source, play_item_id, created_at,
	// updated_at, slot, signature, city, region, country.
	var (
		tuples []string
		args   []any
	)
	for _, p := range plays {
		trackID, err := strconv.Atoi(p.GetTrackId())
		if err != nil {
			continue // non-integer track id: skip (matches Python)
		}

		// Anonymous listens carry a non-integer user_id; store NULL.
		var userID any
		if uid, err := strconv.Atoi(p.GetUserId()); err == nil {
			userID = uid
		} else {
			userID = nil
		}

		n := len(args)
		tuples = append(tuples, fmt.Sprintf("($%d,$%d,$%d,$%d,$%d,$%d,$%d,$%d,$%d,$%d)",
			n+1, n+2, n+3, n+4, n+5, n+6, n+7, n+8, n+9, n+10))
		args = append(args,
			userID,                    // user_id (nullable)
			"relay",                   // source
			trackID,                   // play_item_id
			p.GetTimestamp().AsTime(), // created_at
			now,                       // updated_at
			slot,                      // slot
			p.GetSignature(),          // signature
			p.GetCity(),               // city
			p.GetRegion(),             // region
			p.GetCountry(),            // country
		)
	}

	if len(tuples) == 0 {
		return nil
	}

	sql := `INSERT INTO plays
		(user_id, source, play_item_id, created_at, updated_at, slot, signature, city, region, country)
		VALUES ` + strings.Join(tuples, ",")

	_, err := params.DBTX.Exec(ctx, sql, args...)
	return err
}
