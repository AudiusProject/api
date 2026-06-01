package indexer

import (
	"context"
	"time"

	dbv1 "api.audius.co/api/dbv1"
	"api.audius.co/config"
	"api.audius.co/database"
	"api.audius.co/jobs"
	"api.audius.co/logging"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

type AggregatesCalculator struct {
	logger              *zap.Logger
	readPool            *dbv1.DBPools
	writePool           database.DbPool
	updateAggregatesJob *jobs.UpdateAggregatesJob
}

func NewAggregatesCalculator(config config.Config) *AggregatesCalculator {
	logger := logging.NewZapLogger(config).Named("AggregatesCalculator")
	readPool, err := dbv1.NewDBPools([]string{config.ReadDbUrl}, logger, config.Env, config.ZapLevel)
	if err != nil {
		panic(err)
	}
	writePool, err := pgxpool.New(context.Background(), config.WriteDbUrl)
	if err != nil {
		panic(err)
	}

	return &AggregatesCalculator{
		logger:    logger,
		readPool:  readPool,
		writePool: writePool,
		updateAggregatesJob: jobs.NewUpdateAggregatesJob(jobs.UpdateAggregatesJobConfig{
			WritePool: writePool,
			ReadPool:  readPool,
			Logger:    logger,
		}),
	}
}

// aggregatesPassInterval paces successive full passes of the aggregates
// calculator. A full pass was observed taking ~10 min in production; the old
// `for {}` loop kicked off the next pass the instant the previous one returned,
// so it ran continuously and IO-starved the block-indexing loop. apps' legacy
// `update_aggregates` celery task ran on a fixed 10m beat (src/app.py
// beat_schedule), so we pace to the same cadence: wait until 10m has elapsed
// since the previous pass *started* before beginning the next. Because the work
// is a single serial loop, a pass can never overlap itself, so no mutex guard
// is needed — the pacing alone is what was missing.
const aggregatesPassInterval = 10 * time.Minute

func (a *AggregatesCalculator) Start(ctx context.Context) error {
	a.logger.Info("Starting aggregates calculator")
	go logging.SyncOnTicks(ctx, a.logger, time.Second*10)
	// This job runs in a continuous loop until the context is cancelled, paced
	// at aggregatesPassInterval between the *starts* of consecutive passes. If a
	// pass overruns the interval the next one starts immediately (no negative
	// sleep), so we never fall behind — we just stop hot-looping when passes are
	// fast.
	for {
		passStart := time.Now()
		a.updateAggregatesJob.Run(ctx)

		select {
		case <-ctx.Done():
			a.logger.Info("Shutting down aggregates calculator")
			return ctx.Err()
		default:
		}

		if wait := aggregatesPassInterval - time.Since(passStart); wait > 0 {
			select {
			case <-ctx.Done():
				a.logger.Info("Shutting down aggregates calculator")
				return ctx.Err()
			case <-time.After(wait):
			}
		}
	}
}

func (a *AggregatesCalculator) Close() {
	a.readPool.Close()
	a.writePool.Close()
}
