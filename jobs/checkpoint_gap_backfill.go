package jobs

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"sync"
	"time"

	"api.audius.co/config"
	"api.audius.co/database"
	"api.audius.co/logging"
	"github.com/jackc/pgx/v5"
	"go.uber.org/zap"
)

// Backfillable is satisfied by any indexer that can replay transactions for
// its subscribed addresses across a slot range.
type Backfillable interface {
	Backfill(ctx context.Context, fromSlot, toSlot uint64) error
}

// CheckpointGapBackfillJob scans sol_slot_checkpoints for each registered
// backfillable indexer, identifies uncovered slot ranges (gaps left by
// subscription downtime or earlier-version processing failures), and dispatches
// the corresponding indexer's Backfill method to recover them.
type CheckpointGapBackfillJob struct {
	pool         database.DbPool
	backfillers  map[string]Backfillable
	logger       *zap.Logger
	minGapSize   uint64
	maxGapPerRun int

	mutex     sync.Mutex
	isRunning bool
}

type CheckpointGapBackfillJobConfig struct {
	Pool        database.DbPool
	Backfillers map[string]Backfillable
	// MinGapSize ignores gaps smaller than this many slots. Tiny gaps are
	// common during normal reconnects and aren't worth a full backfill pass.
	MinGapSize uint64
	// MaxGapsPerRun caps how many gaps a single tick will process, so a sudden
	// flood of gaps can't pin the RPC for hours.
	MaxGapsPerRun int
}

func NewCheckpointGapBackfillJob(cfg config.Config, jobCfg CheckpointGapBackfillJobConfig) *CheckpointGapBackfillJob {
	logger := logging.NewZapLogger(cfg).Named("CheckpointGapBackfillJob")
	if jobCfg.MinGapSize == 0 {
		jobCfg.MinGapSize = 2500 // matches common.MAX_SLOT_GAP — anything smaller is a tolerable reconnect
	}
	if jobCfg.MaxGapsPerRun == 0 {
		jobCfg.MaxGapsPerRun = 8
	}
	return &CheckpointGapBackfillJob{
		pool:         jobCfg.Pool,
		backfillers:  jobCfg.Backfillers,
		logger:       logger,
		minGapSize:   jobCfg.MinGapSize,
		maxGapPerRun: jobCfg.MaxGapsPerRun,
	}
}

func (j *CheckpointGapBackfillJob) ScheduleEvery(ctx context.Context, duration time.Duration) *CheckpointGapBackfillJob {
	go func() {
		ticker := time.NewTicker(duration)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				j.Run(ctx)
			case <-ctx.Done():
				j.logger.Info("Job schedule shutting down")
				return
			}
		}
	}()
	return j
}

func (j *CheckpointGapBackfillJob) Run(ctx context.Context) {
	j.logger.Info("Job started")
	if err := j.run(ctx); err != nil {
		j.logger.Error("Job run failed", zap.Error(err))
	} else {
		j.logger.Info("Job completed successfully")
	}
}

func (j *CheckpointGapBackfillJob) run(ctx context.Context) error {
	j.mutex.Lock()
	if j.isRunning {
		j.mutex.Unlock()
		return fmt.Errorf("job is already running")
	}
	j.isRunning = true
	j.mutex.Unlock()
	defer func() {
		j.mutex.Lock()
		j.isRunning = false
		j.mutex.Unlock()
	}()

	names := make([]string, 0, len(j.backfillers))
	for n := range j.backfillers {
		names = append(names, n)
	}
	sort.Strings(names)

	for _, name := range names {
		bf := j.backfillers[name]
		gaps, err := j.findGaps(ctx, name)
		if err != nil {
			j.logger.Error("failed to find gaps", zap.String("name", name), zap.Error(err))
			continue
		}
		if len(gaps) == 0 {
			j.logger.Debug("no gaps detected", zap.String("name", name))
			continue
		}
		processed := 0
		for _, gap := range gaps {
			if processed >= j.maxGapPerRun {
				j.logger.Info("reached max gaps per run, deferring remaining",
					zap.String("name", name),
					zap.Int("remaining", len(gaps)-processed),
				)
				break
			}
			j.logger.Info("backfilling checkpoint gap",
				zap.String("name", name),
				zap.Uint64("fromSlot", gap.from),
				zap.Uint64("toSlot", gap.to),
				zap.Uint64("size", gap.to-gap.from+1),
			)
			if err := bf.Backfill(ctx, gap.from, gap.to); err != nil {
				j.logger.Error("backfill failed",
					zap.String("name", name),
					zap.Uint64("fromSlot", gap.from),
					zap.Uint64("toSlot", gap.to),
					zap.Error(err),
				)
				continue
			}
			if err := j.markGapFilled(ctx, name, gap); err != nil {
				j.logger.Error("failed to record gap-fill checkpoint",
					zap.String("name", name),
					zap.Uint64("fromSlot", gap.from),
					zap.Uint64("toSlot", gap.to),
					zap.Error(err),
				)
			}
			processed++
		}
	}
	return nil
}

type slotRange struct {
	from uint64
	to   uint64
}

func (j *CheckpointGapBackfillJob) findGaps(ctx context.Context, name string) ([]slotRange, error) {
	rows, err := j.pool.Query(ctx, `
		SELECT from_slot, to_slot
		FROM sol_slot_checkpoints
		WHERE name = $1
		ORDER BY from_slot ASC
	`, name)
	if err != nil {
		return nil, fmt.Errorf("query checkpoints: %w", err)
	}
	defer rows.Close()

	var ranges []slotRange
	for rows.Next() {
		var from, to int64
		if err := rows.Scan(&from, &to); err != nil {
			return nil, fmt.Errorf("scan checkpoint row: %w", err)
		}
		if to < from {
			continue
		}
		ranges = append(ranges, slotRange{from: uint64(from), to: uint64(to)})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate checkpoint rows: %w", err)
	}

	merged := mergeRanges(ranges)
	gaps := make([]slotRange, 0)
	for i := 1; i < len(merged); i++ {
		prev := merged[i-1]
		cur := merged[i]
		if cur.from <= prev.to+1 {
			continue
		}
		gap := slotRange{from: prev.to + 1, to: cur.from - 1}
		if gap.to-gap.from+1 < j.minGapSize {
			continue
		}
		gaps = append(gaps, gap)
	}
	return gaps, nil
}

// mergeRanges collapses overlapping/adjacent slot ranges. Caller passes
// ranges sorted by `from`.
func mergeRanges(ranges []slotRange) []slotRange {
	if len(ranges) == 0 {
		return nil
	}
	merged := make([]slotRange, 0, len(ranges))
	cur := ranges[0]
	for i := 1; i < len(ranges); i++ {
		next := ranges[i]
		if next.from <= cur.to+1 {
			if next.to > cur.to {
				cur.to = next.to
			}
			continue
		}
		merged = append(merged, cur)
		cur = next
	}
	merged = append(merged, cur)
	return merged
}

// markGapFilled inserts a checkpoint row claiming coverage for a backfilled gap,
// so subsequent runs of this job see the slot range as covered.
func (j *CheckpointGapBackfillJob) markGapFilled(ctx context.Context, name string, gap slotRange) error {
	subscription := fmt.Sprintf(`{"type":"gap-backfill","name":%q,"fromSlot":%d,"toSlot":%d}`, name, gap.from, gap.to)
	sum := sha256.Sum256([]byte(subscription))
	hash := hex.EncodeToString(sum[:])
	_, err := j.pool.Exec(ctx, `
		INSERT INTO sol_slot_checkpoints (name, from_slot, to_slot, subscription, subscription_hash)
		VALUES (@name, @from_slot, @to_slot, @subscription, @subscription_hash)
	`, pgx.NamedArgs{
		"name":              name,
		"from_slot":         gap.from,
		"to_slot":           gap.to,
		"subscription":      subscription,
		"subscription_hash": hash,
	})
	return err
}
