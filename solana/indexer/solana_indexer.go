package indexer

import (
	"context"
	"fmt"
	"time"

	"api.audius.co/config"
	"api.audius.co/database"
	"api.audius.co/logging"
	"api.audius.co/solana/indexer/common"
	"api.audius.co/solana/indexer/locker"
	"api.audius.co/solana/indexer/program"
	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/maypok86/otter"
	pb "github.com/rpcpool/yellowstone-grpc/examples/golang/proto"
	"go.uber.org/zap"
)

type Indexer interface {
	Start(ctx context.Context)
	HandleUpdate(ctx context.Context, updateMessage *pb.SubscribeUpdate) error
}

type SolanaIndexer struct {
	rpcClient common.RpcClient

	config      config.Config
	pool        database.DbPool
	workerCount int32

	indexers map[string]Indexer

	logger *zap.Logger
}

// Creates a Solana indexer.
func New(config config.Config) *SolanaIndexer {
	logger := logging.NewZapLogger(config).
		Named("SolanaIndexer")

	rpcClient := rpc.New(config.SolanaConfig.RpcProviders[0])

	connConfig, err := pgxpool.ParseConfig(config.WriteDbUrl)
	if err != nil {
		panic(fmt.Errorf("error parsing database URL: %w", err))
	}

	// The min write pool size is set to the number of workers
	// plus 1 for the connection that listens for artist_coins changes,
	// and add 10 as a buffer.
	workerCount := int32(config.SolanaIndexerWorkers)
	connConfig.MaxConns = workerCount + 1 + 10

	pool, err := pgxpool.NewWithConfig(context.Background(), connConfig)
	if err != nil {
		panic(fmt.Errorf("error connecting to database: %w", err))
	}

	grpcConfig := common.GrpcConfig{
		Server:                config.SolanaConfig.GrpcProvider,
		ApiToken:              config.SolanaConfig.GrpcToken,
		MaxReconnectAttempts:  5,
		UseFumarole:           config.SolanaConfig.UseFumarole,
		FumaroleConsumerGroup: config.SolanaConfig.FumaroleConsumerGroup,
	}

	transactionCache, err := otter.MustBuilder[solana.Signature, *rpc.GetTransactionResult](50).
		WithTTL(30 * time.Second).
		CollectStats().
		Build()

	if err != nil {
		panic(fmt.Errorf("failed to create transaction cache: %w", err))
	}

	// dammV2Indexer := damm_v2.New(
	// 	grpcConfig, rpcClient, pool, &transactionCache, logger,
	// )
	// tokenIndexer := token.New(
	// 	grpcConfig, rpcClient, pool, &transactionCache, logger,
	// )
	programIndexer := program.New(
		grpcConfig, rpcClient, pool, config, &transactionCache, logger,
	)
	// dbcIndexer := dbc.New(
	// 	grpcConfig, rpcClient, pool, config, &transactionCache, logger,
	// )
	// lockerIndexer := locker.New(
	// 	grpcConfig, rpcClient, pool, logger,
	// )

	indexers := make(map[string]Indexer)
	// indexers[damm_v2.NAME] = dammV2Indexer
	// indexers[token.NAME] = tokenIndexer
	indexers[program.NAME] = programIndexer
	// indexers[dbc.NAME] = dbcIndexer
	// indexers[locker.NAME] = lockerIndexer

	s := &SolanaIndexer{
		rpcClient:   rpcClient,
		logger:      logger,
		config:      config,
		pool:        pool,
		workerCount: workerCount,
		indexers:    indexers,
	}

	return s
}

func (s *SolanaIndexer) Start(ctx context.Context) error {
	go s.ScheduleProcessRetryQueue(ctx, s.config.SolanaIndexerRetryInterval)

	// statsJob := jobs.NewCoinStatsJob(s.config, s.pool)
	// statsCtx := context.WithoutCancel(ctx)
	// statsJob.ScheduleEvery(statsCtx, 15*time.Minute)
	// go statsJob.Run(statsCtx)

	// audioPriceJob := jobs.NewAudioPriceJob(s.config, s.pool)
	// priceCtx := context.WithoutCancel(ctx)
	// audioPriceJob.ScheduleEvery(priceCtx, 5*time.Minute)
	// go audioPriceJob.Run(priceCtx)

	// dbcJob := jobs.NewCoinDBCJob(s.config, s.pool)
	// dbcCtx := context.WithoutCancel(ctx)
	// dbcJob.ScheduleEvery(dbcCtx, 1*time.Minute)
	// go dbcJob.Run(dbcCtx)

	// balanceHistoryJob := jobs.NewBalanceHistoryJob(s.config, s.pool)
	// balanceHistoryCtx := context.WithoutCancel(ctx)
	// balanceHistoryJob.ScheduleEvery(balanceHistoryCtx, 1*time.Hour)
	// go balanceHistoryJob.Run(balanceHistoryCtx)

	for _, indexer := range s.indexers {
		go indexer.Start(ctx)
	}

	for {
		select {
		case <-ctx.Done():
			s.logger.Info("received shutdown signal, stopping solana indexer")
			return nil
		default:
		}
	}
}

func (s *SolanaIndexer) ScheduleProcessRetryQueue(ctx context.Context, interval time.Duration) {
	s.logger.Debug("starting retry ticker", zap.Duration("interval", interval))
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			s.logger.Debug("stopping retry ticker")
			return
		case <-ticker.C:
			s.ProcessRetryQueue(ctx)
		}
	}
}

func (s *SolanaIndexer) ProcessRetryQueue(ctx context.Context) {
	limit := 100
	offset := 0
	logger := s.logger.Named("RetryQueue")
	count := 0
	start := time.Now()
	logger.Debug("starting to process retry queue...")
	for {
		queue, err := common.GetRetryQueue(ctx, s.pool, limit, offset)
		if err != nil {
			logger.Error("failed to fetch retry queue", zap.Error(err))
			return
		}
		if len(queue) == 0 {
			break
		}

		for _, item := range queue {
			indexer := s.indexers[item.Indexer]
			if indexer == nil {
				logger.Warn("unknown indexer in retry queue", zap.String("indexer", item.Indexer))
				offset++
				continue
			}
			err := indexer.HandleUpdate(ctx, item.UpdateMessage.SubscribeUpdate)
			if err != nil {
				logger.Error("failed to retry", zap.String("indexer", locker.NAME), zap.Error(err))
				offset++
			} else {
				err = common.DeleteFromRetryQueue(ctx, s.pool, item.ID)
				if err != nil {
					logger.Error("failed to delete from retry queue", zap.Error(err))
				}
			}
			count++
		}
	}

	if count == 0 {
		logger.Debug("no unprocessed transactions to retry")
		return
	}

	logger.Info("finished processing retry queue",
		zap.Int("count", count),
		zap.Int("failed", offset),
		zap.Duration("duration", time.Since(start)),
	)
}

type solanaHealth struct {
	ChainSlot uint64             `json:"chain_slot"`
	Errors    []string           `json:"errors,omitempty"`
	Indexers  []indexerHealthRow `json:"indexers"`
}

type indexerHealthRow struct {
	Name            string     `json:"name"`
	SlotDiff        uint64     `json:"slot_diff"`
	IndexedSlot     uint64     `json:"indexed_slot"`
	RetryQueueCount int        `json:"retry_queue_count"`
	UpdatedAt       *time.Time `json:"updated_at"`
}

func (s *SolanaIndexer) GetHealth(ctx context.Context, maxSlotDiff uint64, maxRetryQueue int) (*solanaHealth, error) {
	chainSlot, err := s.rpcClient.GetSlot(ctx, rpc.CommitmentConfirmed)
	if err != nil {
		return nil, fmt.Errorf("failed to get chain slot: %w", err)
	}

	names := make([]string, 0, len(s.indexers))
	for name := range s.indexers {
		names = append(names, name)
	}

	sql := `
		WITH retry_queue_by_indexer AS (
			SELECT
				indexer,
				COUNT(*) AS retry_queue_count
			FROM sol_retry_queue
			GROUP BY indexer
		) SELECT DISTINCT ON (indexers.name) 
			indexers.name,
			to_slot AS indexed_slot,
			GREATEST(@chain_slot - to_slot, 0) AS slot_diff,
			COALESCE(retry_queue_count, 0) AS retry_queue_count,
			updated_at
		FROM UNNEST(@indexers::TEXT[]) AS indexers(name)
		LEFT JOIN sol_slot_checkpoints ON sol_slot_checkpoints.name = indexers.name
		LEFT JOIN retry_queue_by_indexer ON indexer = indexers.name
		ORDER BY indexers.name, from_slot DESC NULLS LAST
	;`

	rows, err := s.pool.Query(ctx, sql, pgx.NamedArgs{
		"chain_slot": chainSlot,
		"indexers":   names,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to query indexer health: %w", err)
	}

	healths, err := pgx.CollectRows(rows, pgx.RowToStructByName[indexerHealthRow])
	if err != nil {
		return nil, fmt.Errorf("failed to collect indexer health rows: %w", err)
	}

	errors := make([]string, 0)
	for _, h := range healths {
		if h.RetryQueueCount > maxRetryQueue {
			errors = append(errors, fmt.Sprintf("indexer %s has high retry queue count: %d", h.Name, h.RetryQueueCount))
		}

		if h.SlotDiff > maxSlotDiff {
			errors = append(errors, fmt.Sprintf("indexer %s has high slot diff: %d", h.Name, h.SlotDiff))
		}
	}

	return &solanaHealth{
		ChainSlot: chainSlot,
		Errors:    errors,
		Indexers:  healths,
	}, nil
}

func (s *SolanaIndexer) Close() {
	s.pool.Close()
}
