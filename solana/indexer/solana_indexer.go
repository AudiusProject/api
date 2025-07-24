package indexer

import (
	"context"
	"fmt"
	"time"

	"bridgerton.audius.co/config"
	"bridgerton.audius.co/logging"
	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	pb "github.com/rpcpool/yellowstone-grpc/examples/golang/proto"
	"go.uber.org/zap"
)

type DbPool interface {
	Acquire(context.Context) (*pgxpool.Conn, error)
	Begin(context.Context) (pgx.Tx, error)
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
	Query(context.Context, string, ...any) (pgx.Rows, error)
	QueryRow(context.Context, string, ...any) pgx.Row
	Close()
}

type RpcClient interface {
	GetBlockWithOpts(context.Context, uint64, *rpc.GetBlockOpts) (*rpc.GetBlockResult, error)
	GetSlot(context.Context, rpc.CommitmentType) (uint64, error)
	GetSignaturesForAddressWithOpts(context.Context, solana.PublicKey, *rpc.GetSignaturesForAddressOpts) ([]*rpc.TransactionSignature, error)
	GetTransaction(context.Context, solana.Signature, *rpc.GetTransactionOpts) (*rpc.GetTransactionResult, error)
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
	pool        DbPool
	flushTicker *time.Ticker

	checkpointId string

	logger *zap.Logger
}

// Creates a Solana indexer.
func New(config config.Config) *SolanaIndexer {
	logger := logging.NewZapLogger(config).
		With(zap.String("service", "SolanaIndexer"))

	rpcClient := rpc.New(config.SolanaConfig.RpcProviders[0])

	connConfig, err := pgxpool.ParseConfig(config.WriteDbUrl)
	if err != nil {
		panic(fmt.Errorf("error parsing database URL: %w", err))
	}

	pool, err := pgxpool.NewWithConfig(context.Background(), connConfig)
	if err != nil {
		panic(fmt.Errorf("error connecting to database: %w", err))
	}

	grpcClient := NewGrpcClient(GrpcConfig{
		Server:               config.SolanaConfig.GrpcProvider,
		ApiToken:             config.SolanaConfig.GrpcToken,
		MaxReconnectAttempts: 5,
	})

	s := &SolanaIndexer{
		rpcClient:   rpcClient,
		grpcClient:  grpcClient,
		logger:      logger,
		config:      config,
		pool:        pool,
		flushTicker: time.NewTicker(time.Second * 15),
		processor: NewDefaultProcessor(
			rpcClient,
			pool,
			config,
		),
	}

	go func() {
		for range s.flushTicker.C {
			s.syncLogs()
		}
	}()

	return s
}

func (s *SolanaIndexer) syncLogs() {
	err := s.logger.Sync()
	if err != nil {
		s.logger.Error("failed to sync logger", zap.Error(err))
	}
}

func (s *SolanaIndexer) Close() {
	if p, ok := s.processor.(*DefaultProcessor); ok {
		p.ReportCacheStats(s.logger)
	}
	s.syncLogs()
	s.flushTicker.Stop()
	s.grpcClient.Close()
	s.pool.Close()
}
