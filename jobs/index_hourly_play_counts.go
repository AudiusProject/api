package jobs

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"api.audius.co/config"
	"api.audius.co/database"
	"api.audius.co/logging"
	"github.com/jackc/pgx/v5"
	"go.uber.org/zap"
)

// HourlyPlayCountsJob rolls up plays into the hourly_play_counts table.
// Mirrors apps/packages/discovery-provider/src/tasks/index_hourly_play_counts.py.
//
// On each run:
//   - reads the last processed plays.id checkpoint from indexing_checkpoints,
//   - buckets newer plays by date_trunc('hour', created_at),
//   - upserts into hourly_play_counts with sum-on-conflict,
//   - advances the checkpoint to max(plays.id) seen.
//
// hourly_play_counts powers GET /v1/metrics/plays. If this job stops running
// the endpoint returns an empty array (it doesn't error).
type HourlyPlayCountsJob struct {
	pool   database.DbPool
	logger *zap.Logger

	mutex     sync.Mutex
	isRunning bool
}

// HourlyPlayCountsCheckpoint is the indexing_checkpoints.tablename used to
// remember the highest plays.id already rolled up.
const HourlyPlayCountsCheckpoint = "hourly_play_counts"

func NewHourlyPlayCountsJob(cfg config.Config, pool database.DbPool) *HourlyPlayCountsJob {
	return &HourlyPlayCountsJob{
		pool:   pool,
		logger: logging.NewZapLogger(cfg).Named("HourlyPlayCountsJob"),
	}
}

// ScheduleEvery runs the job every `interval` until the context is cancelled.
func (j *HourlyPlayCountsJob) ScheduleEvery(ctx context.Context, interval time.Duration) *HourlyPlayCountsJob {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				j.Run(ctx)
			case <-ctx.Done():
				j.logger.Info("Job shutting down")
				return
			}
		}
	}()
	return j
}

// Run executes the job once.
func (j *HourlyPlayCountsJob) Run(ctx context.Context) {
	if err := j.run(ctx); err != nil {
		j.logger.Error("Job run failed", zap.Error(err))
	}
}

func (j *HourlyPlayCountsJob) run(ctx context.Context) error {
	start := time.Now()
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

	prev, err := getCheckpoint(ctx, j.pool, HourlyPlayCountsCheckpoint)
	if err != nil {
		return fmt.Errorf("read checkpoint: %w", err)
	}

	var newMax *int64
	err = j.pool.QueryRow(ctx, "SELECT MAX(id) FROM plays").Scan(&newMax)
	if err != nil {
		return fmt.Errorf("read max(plays.id): %w", err)
	}
	if newMax == nil || *newMax == prev {
		j.logger.Debug("No new plays since last run",
			zap.Duration("duration", time.Since(start)))
		return nil
	}

	// Bucket and upsert in a single statement: the SUM-on-conflict means we
	// can safely process the same row twice (idempotent on full batches),
	// but the checkpoint advances per run so the steady-state cost stays
	// bounded.
	res, err := j.pool.Exec(ctx, `
		INSERT INTO hourly_play_counts (hourly_timestamp, play_count)
		SELECT date_trunc('hour', created_at) AS hourly_timestamp,
		       COUNT(*) AS play_count
		FROM plays
		WHERE id > @prev AND id <= @new_max
		GROUP BY date_trunc('hour', created_at)
		ON CONFLICT (hourly_timestamp)
		DO UPDATE SET play_count = hourly_play_counts.play_count + EXCLUDED.play_count
	`, pgx.NamedArgs{
		"prev":    prev,
		"new_max": *newMax,
	})
	if err != nil {
		return fmt.Errorf("upsert hourly buckets: %w", err)
	}

	if err := saveCheckpoint(ctx, j.pool, HourlyPlayCountsCheckpoint, *newMax); err != nil {
		return fmt.Errorf("save checkpoint: %w", err)
	}

	j.logger.Info("Hourly play counts updated",
		zap.Int64("prev_checkpoint", prev),
		zap.Int64("new_checkpoint", *newMax),
		zap.Int64("hours_touched", res.RowsAffected()),
		zap.Duration("duration", time.Since(start)))
	return nil
}

// getCheckpoint reads the named checkpoint value (last_checkpoint column),
// returning 0 when the row does not yet exist.
func getCheckpoint(ctx context.Context, pool database.DbPool, name string) (int64, error) {
	var v int64
	err := pool.QueryRow(ctx, "SELECT last_checkpoint FROM indexing_checkpoints WHERE tablename = $1", name).Scan(&v)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, nil
	}
	return v, err
}

// saveCheckpoint upserts the named checkpoint.
func saveCheckpoint(ctx context.Context, pool database.DbPool, name string, value int64) error {
	_, err := pool.Exec(ctx, `
		INSERT INTO indexing_checkpoints (tablename, last_checkpoint)
		VALUES ($1, $2)
		ON CONFLICT (tablename) DO UPDATE SET last_checkpoint = EXCLUDED.last_checkpoint
	`, name, value)
	return err
}
