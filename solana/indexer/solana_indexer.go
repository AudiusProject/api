package indexer

import (
	"context"
	"fmt"

	"api.audius.co/config"
	"api.audius.co/database"
	"api.audius.co/logging"
	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
	"github.com/jackc/pgx/v5/pgxpool"
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

type SolanaIndexer struct {
	rpcClient  RpcClient
	grpcClient GrpcClient
	processor  Processor

	config      config.Config
	pool        database.DbPool
	workerCount int32

	dammV2Indexer *DammV2Indexer

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

	grpcClient := NewGrpcClient(GrpcConfig{
		Server:               config.SolanaConfig.GrpcProvider,
		ApiToken:             config.SolanaConfig.GrpcToken,
		MaxReconnectAttempts: 5,
	})

	dammV2Indexer := &DammV2Indexer{
		pool: pool,
		grpcConfig: GrpcConfig{
			Server:               config.SolanaConfig.GrpcProvider,
			ApiToken:             config.SolanaConfig.GrpcToken,
			MaxReconnectAttempts: 5,
		},
		logger: logger,
	}

	s := &SolanaIndexer{
		rpcClient:   rpcClient,
		grpcClient:  grpcClient,
		logger:      logger,
		config:      config,
		pool:        pool,
		workerCount: workerCount,

		dammV2Indexer: dammV2Indexer,

		processor: NewDefaultProcessor(
			rpcClient,
			pool,
			config,
		),
	}

	return s
}

func (s *SolanaIndexer) Start(ctx context.Context) error {
	go s.ScheduleRetries(ctx, s.config.SolanaIndexerRetryInterval)

	// statsJob := jobs.NewCoinStatsJob(s.config, s.pool)
	// statsCtx := context.WithoutCancel(ctx)
	// statsJob.ScheduleEvery(statsCtx, 5*time.Minute)
	// go statsJob.Run(statsCtx)

	// dbcJob := jobs.NewCoinDBCJob(s.config, s.pool)
	// dbcCtx := context.WithoutCancel(ctx)
	// dbcJob.ScheduleEvery(dbcCtx, 5*time.Minute)
	// go dbcJob.Run(dbcCtx)

	go s.dammV2Indexer.Start(ctx)

	err := s.Subscribe(ctx)
	if err != nil {
		return fmt.Errorf("failed to subscribe: %w", err)
	}

	return nil
}

func (s *SolanaIndexer) Close() {
	if p, ok := s.processor.(*DefaultProcessor); ok {
		p.ReportCacheStats(s.logger)
	}
	s.grpcClient.Close()
	s.pool.Close()
}
