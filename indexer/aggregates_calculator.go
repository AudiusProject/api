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

func (a *AggregatesCalculator) Start(ctx context.Context) error {
	a.logger.Info("Starting aggregates calculator")
	go logging.SyncOnTicks(ctx, a.logger, time.Second*10)
	// This job runs in a continous loop until the context is cancelled.
	for {
		select {
		case <-ctx.Done():
			a.logger.Info("Shutting down aggregates calculator")
			return ctx.Err()
		default:
			a.updateAggregatesJob.Run(ctx)
		}
	}
}

func (a *AggregatesCalculator) Close() {
	a.readPool.Close()
	a.writePool.Close()
}
