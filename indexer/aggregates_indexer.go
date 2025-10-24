package indexer

import (
	"context"

	dbv1 "api.audius.co/api/dbv1"
	"api.audius.co/config"
	"api.audius.co/database"
	"api.audius.co/jobs"
	"api.audius.co/logging"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

type AggregatesIndexer struct {
	logger              *zap.Logger
	readPool            *dbv1.DBPools
	writePool           database.DbPool
	updateAggregatesJob *jobs.UpdateAggregatesJob
}

func NewAggregatesIndexer(config config.Config) *AggregatesIndexer {
	logger := logging.NewZapLogger(config).Named("AggregatesIndexer")
	readPool, err := dbv1.NewDBPools([]string{config.ReadDbUrl}, logger, config.Env, config.ZapLevel)
	if err != nil {
		panic(err)
	}
	writePool, err := pgxpool.New(context.Background(), config.WriteDbUrl)
	if err != nil {
		panic(err)
	}

	return &AggregatesIndexer{
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

func (a *AggregatesIndexer) Start(ctx context.Context) error {
	a.logger.Info("Starting aggregates indexer")
	// This job runs in a continous loop until the context is cancelled.
	for {
		select {
		case <-ctx.Done():
			a.logger.Info("Shutting down aggregates indexer")
			return ctx.Err()
		default:
			a.updateAggregatesJob.Run(ctx)
		}
	}
}

func (a *AggregatesIndexer) Close() {
	a.readPool.Close()
	a.writePool.Close()
}
