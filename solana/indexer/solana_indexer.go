package indexer

import (
	"context"
	"fmt"
	"time"

	"api.audius.co/config"
	"api.audius.co/database"
	"api.audius.co/jobs"
	"api.audius.co/logging"
	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/maypok86/otter"
	pb "github.com/rpcpool/yellowstone-grpc/examples/golang/proto"
	"go.uber.org/zap"
)

type RpcClient interface {
	GetBlockWithOpts(context.Context, uint64, *rpc.GetBlockOpts) (*rpc.GetBlockResult, error)
	GetSlot(context.Context, rpc.CommitmentType) (uint64, error)
	GetSignaturesForAddressWithOpts(context.Context, solana.PublicKey, *rpc.GetSignaturesForAddressOpts) ([]*rpc.TransactionSignature, error)
	GetTransaction(context.Context, solana.Signature, *rpc.GetTransactionOpts) (*rpc.GetTransactionResult, error)
	GetAccountDataBorshInto(ctx context.Context, account solana.PublicKey, out interface{}) error
}

type GrpcClient interface {
	Subscribe(
		ctx context.Context,
		subRequest *pb.SubscribeRequest,
		dataCallback DataCallback,
		errorCallback ErrorCallback,
	) error
	Close()
}

const MAX_SLOT_GAP = 2500

type SolanaIndexer struct {
	rpcClient  RpcClient
	grpcClient GrpcClient
	processor  Processor

	config      config.Config
	pool        database.DbPool
	workerCount int32

	dammV2Indexer *DammV2Indexer
	tokenIndexer  *TokenIndexer

	checkpointId string

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

	grpcConfig := GrpcConfig{
		Server:               config.SolanaConfig.GrpcProvider,
		ApiToken:             config.SolanaConfig.GrpcToken,
		MaxReconnectAttempts: 5,
	}

	transactionCache, err := otter.MustBuilder[solana.Signature, *rpc.GetTransactionResult](50).
		WithTTL(30 * time.Second).
		CollectStats().
		Build()

	if err != nil {
		panic(fmt.Errorf("failed to create transaction cache: %w", err))
	}

	dammV2Indexer := &DammV2Indexer{
		pool:       pool,
		grpcConfig: grpcConfig,
		rpcClient:  rpcClient,
		logger:     logger.Named("DammV2Indexer"),
	}

	tokenIndexer := &TokenIndexer{
		pool:             pool,
		grpcConfig:       grpcConfig,
		rpcClient:        rpcClient,
		logger:           logger.Named("TokenIndexer"),
		transactionCache: &transactionCache,
	}

	s := &SolanaIndexer{
		rpcClient:   rpcClient,
		logger:      logger,
		config:      config,
		pool:        pool,
		workerCount: workerCount,

		dammV2Indexer: dammV2Indexer,
		tokenIndexer:  tokenIndexer,
	}

	return s
}

func (s *SolanaIndexer) Start(ctx context.Context) error {
	go s.ScheduleProcessRetryQueue(ctx, s.config.SolanaIndexerRetryInterval)

	statsJob := jobs.NewCoinStatsJob(s.config, s.pool)
	statsCtx := context.WithoutCancel(ctx)
	statsJob.ScheduleEvery(statsCtx, 5*time.Minute)
	go statsJob.Run(statsCtx)

	dbcJob := jobs.NewCoinDBCJob(s.config, s.pool)
	dbcCtx := context.WithoutCancel(ctx)
	dbcJob.ScheduleEvery(dbcCtx, 5*time.Minute)
	go dbcJob.Run(dbcCtx)

	go s.tokenIndexer.Start(ctx)
	go s.dammV2Indexer.Start(ctx)

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
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			s.logger.Info("context cancelled, stopping retry ticker")
			return
		case <-ticker.C:
			err := s.ProcessRetryQueue(ctx)
			if err != nil {
				s.logger.Error("failed to retry unprocessed transactions", zap.Error(err))
			}
		}
	}
}

func (s *SolanaIndexer) ProcessRetryQueue(ctx context.Context) error {
	limit := 100
	offset := 0
	logger := s.logger.Named("RetryQueue")
	count := 0
	start := time.Now()
	for {
		queue, err := getRetryQueue(ctx, s.pool, limit, offset)
		if err != nil {
			return fmt.Errorf("failed to fetch retry queue: %w", err)
		}
		if len(queue) == 0 {
			break
		}

		for _, item := range queue {
			switch item.Indexer {
			case "token":
				err := s.tokenIndexer.HandleUpdate(ctx, item.Update.SubscribeUpdate)
				if err != nil {
					logger.Error("failed to retry token_indexer", zap.Error(err))
					offset++
				} else {
					err = deleteFromRetryQueue(ctx, s.pool, item.ID)
					if err != nil {
						logger.Error("failed to delete from retry queue", zap.Error(err))
					}
				}
			case "dammv2":
				err := s.dammV2Indexer.HandleUpdate(ctx, item.Update.SubscribeUpdate)
				if err != nil {
					logger.Error("failed to retry damm_v2_indexer", zap.Error(err))
					offset++
				} else {
					err = deleteFromRetryQueue(ctx, s.pool, item.ID)
					if err != nil {
						logger.Error("failed to delete from retry queue", zap.Error(err))
					}
				}
			default:
				logger.Warn("unknown indexer in retry queue", zap.String("indexer", item.Indexer))
			}
			count++
		}
	}

	if count == 0 {
		logger.Debug("no unprocessed transactions to retry")
		return nil
	}

	logger.Info("finished processing retry queue",
		zap.Int("count", count),
		zap.Int("failed", offset),
		zap.Duration("duration", time.Since(start)),
	)
	return nil
}

func (s *SolanaIndexer) Close() {
	s.pool.Close()
}
