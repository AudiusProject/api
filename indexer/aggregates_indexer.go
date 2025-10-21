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
)

type AggregatesIndexer struct {
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
		readPool:            readPool,
		writePool:           writePool,
		updateAggregatesJob: jobs.NewUpdateAggregatesJob(config, writePool, readPool),
	}
}

func (a *AggregatesIndexer) Start(ctx context.Context) error {
	// try to run every 5 minutes. Job may take longer than 5 minutes to complete
	// and result in every 10 minutes.
	a.updateAggregatesJob.ScheduleEvery(ctx, 5*time.Minute)
	go a.updateAggregatesJob.Run(ctx)

	// wait on context to be cancelled
	<-ctx.Done()
	return ctx.Err()
}

func (a *AggregatesIndexer) Close() {
	a.readPool.Close()
	a.writePool.Close()
}
