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

// CityPlaysJob rolls plays up into aggregate_daily_city_plays, which serves
// GET /v1/users/:id/listener_cities.
//
// Each run reads plays after the last checkpointed plays.id, at most
// batchSize ids at a time so the first runs can backfill the retained plays
// without one huge statement. Plays with no city and plays by the track's
// owner are skipped, matching the weekly artist recap.
type CityPlaysJob struct {
	pool      database.DbPool
	logger    *zap.Logger
	batchSize int64

	mutex     sync.Mutex
	isRunning bool
}

const (
	CityPlaysCheckpoint = "aggregate_daily_city_plays"
	CityPlaysBatchSize  = 250_000
)

func NewCityPlaysJob(cfg config.Config, pool database.DbPool) *CityPlaysJob {
	return &CityPlaysJob{
		pool:      pool,
		logger:    logging.NewZapLogger(cfg).Named("CityPlaysJob"),
		batchSize: CityPlaysBatchSize,
	}
}

// WithBatchSize overrides the per-run id range (for tests).
func (j *CityPlaysJob) WithBatchSize(n int64) *CityPlaysJob {
	j.batchSize = n
	return j
}

func (j *CityPlaysJob) ScheduleEvery(ctx context.Context, interval time.Duration) *CityPlaysJob {
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

func (j *CityPlaysJob) Run(ctx context.Context) {
	if err := j.run(ctx); err != nil {
		j.logger.Error("Job run failed", zap.Error(err))
	}
}

func (j *CityPlaysJob) run(ctx context.Context) error {
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

	prev, err := getCheckpoint(ctx, j.pool, CityPlaysCheckpoint)
	if err != nil {
		return fmt.Errorf("read checkpoint: %w", err)
	}

	var minId, maxId *int64
	if err := j.pool.QueryRow(ctx, "SELECT MIN(id), MAX(id) FROM plays").Scan(&minId, &maxId); err != nil {
		return fmt.Errorf("read plays id range: %w", err)
	}
	if maxId == nil || *maxId <= prev {
		return nil
	}
	// Skip ids that have already been pruned.
	if *minId-1 > prev {
		prev = *minId - 1
	}
	end := min(prev+j.batchSize, *maxId)

	res, err := j.pool.Exec(ctx, `
		INSERT INTO aggregate_daily_city_plays (play_item_id, day, country, region, city, count)
		SELECT
			p.play_item_id,
			p.created_at::date,
			COALESCE(TRIM(p.country), ''),
			COALESCE(TRIM(p.region), ''),
			TRIM(p.city),
			COUNT(*)
		FROM plays p
		JOIN tracks t ON t.track_id = p.play_item_id AND t.is_current
		WHERE p.id > @prev AND p.id <= @end
			AND NULLIF(TRIM(p.city), '') IS NOT NULL
			AND p.user_id IS DISTINCT FROM t.owner_id
		GROUP BY 1, 2, 3, 4, 5
		ON CONFLICT (play_item_id, day, country, region, city)
		DO UPDATE SET count = aggregate_daily_city_plays.count + EXCLUDED.count
	`, pgx.NamedArgs{"prev": prev, "end": end})
	if err != nil {
		return fmt.Errorf("upsert city plays: %w", err)
	}

	if err := saveCheckpoint(ctx, j.pool, CityPlaysCheckpoint, end); err != nil {
		return fmt.Errorf("save checkpoint: %w", err)
	}

	j.logger.Debug("City plays updated",
		zap.Int64("prev_checkpoint", prev),
		zap.Int64("new_checkpoint", end),
		zap.Int64("rows", res.RowsAffected()),
		zap.Duration("duration", time.Since(start)))
	return nil
}
