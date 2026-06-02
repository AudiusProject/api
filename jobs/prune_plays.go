package jobs

import (
	"context"
	"fmt"
	"sync"
	"time"

	"api.audius.co/config"
	"api.audius.co/database"
	"api.audius.co/logging"
	"github.com/jackc/pgx/v5"
	"go.uber.org/zap"
)

// PrunePlaysJob deletes old rows from the plays table to bound table size.
// Mirrors apps/packages/discovery-provider/src/tasks/prune_plays.py.
//
// Each run deletes up to PrunePlaysBatchSize plays older than
// (now - PrunePlaysAge). At the apps default schedule (30s interval),
// this can prune ~144k plays per hour.
//
// Important: deletes happen in oldest-first order so backlog is drained
// monotonically; if the job lags, the next run picks up where it left off.
type PrunePlaysJob struct {
	pool   database.DbPool
	logger *zap.Logger

	// age is how old a play has to be before it's eligible for pruning.
	// Configurable to make testing reasonable; production uses
	// PrunePlaysAge.
	age time.Duration
	// batchSize caps the per-run delete. Configurable for the same reason.
	batchSize int

	mutex     sync.Mutex
	isRunning bool
}

const (
	// PrunePlaysAge matches apps: rows older than ~400 days are pruned.
	PrunePlaysAge = 400 * 24 * time.Hour
	// PrunePlaysBatchSize matches apps' DEFAULT_MAX_BATCH.
	PrunePlaysBatchSize = 50_000
)

func NewPrunePlaysJob(cfg config.Config, pool database.DbPool) *PrunePlaysJob {
	return &PrunePlaysJob{
		pool:      pool,
		logger:    logging.NewZapLogger(cfg).Named("PrunePlaysJob"),
		age:       PrunePlaysAge,
		batchSize: PrunePlaysBatchSize,
	}
}

// WithAge overrides the prune age (for tests).
func (j *PrunePlaysJob) WithAge(age time.Duration) *PrunePlaysJob {
	j.age = age
	return j
}

// WithBatchSize overrides the per-run cap (for tests).
func (j *PrunePlaysJob) WithBatchSize(n int) *PrunePlaysJob {
	j.batchSize = n
	return j
}

// ScheduleEvery runs the job every `interval` until the context is cancelled.
func (j *PrunePlaysJob) ScheduleEvery(ctx context.Context, interval time.Duration) *PrunePlaysJob {
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
func (j *PrunePlaysJob) Run(ctx context.Context) {
	if err := j.run(ctx); err != nil {
		j.logger.Error("Job run failed", zap.Error(err))
	}
}

func (j *PrunePlaysJob) run(ctx context.Context) error {
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

	cutoff := time.Now().Add(-j.age)

	res, err := j.pool.Exec(ctx, `
		WITH to_prune AS (
		    SELECT id
		    FROM plays
		    WHERE created_at <= @cutoff
		    ORDER BY created_at ASC
		    LIMIT @batch
		)
		DELETE FROM plays
		WHERE id IN (SELECT id FROM to_prune)
	`, pgx.NamedArgs{
		"cutoff": cutoff,
		"batch":  j.batchSize,
	})
	if err != nil {
		return fmt.Errorf("delete plays: %w", err)
	}

	deleted := res.RowsAffected()
	if deleted > 0 {
		j.logger.Info("Pruned plays",
			zap.Int64("deleted", deleted),
			zap.Time("cutoff", cutoff),
			zap.Int("batch_size", j.batchSize),
			zap.Duration("duration", time.Since(start)))
	} else {
		j.logger.Debug("No plays pruned",
			zap.Time("cutoff", cutoff),
			zap.Int("batch_size", j.batchSize),
			zap.Duration("duration", time.Since(start)))
	}
	return nil
}
