package indexer

import (
	"context"
	"fmt"

	"bridgerton.audius.co/config"
	"bridgerton.audius.co/logging"
	"github.com/gagliardetto/solana-go/rpc"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

type SolanaIndexer struct {
	rpcClient  *rpc.Client
	grpcClient *GrpcClient

	config config.Config
	pool   *pgxpool.Pool

	checkpointId string
	mintsFilter  []string

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

	return &SolanaIndexer{
		rpcClient:  rpcClient,
		grpcClient: grpcClient,
		logger:     logger,
		config:     config,
		pool:       pool,
	}
}

func (s *SolanaIndexer) Close() error {
	s.pool.Close()
	return nil
}
