package meteora_dbc

import (
	"context"

	"github.com/gagliardetto/solana-go"
	"go.uber.org/zap"
)

var DbcProgramID = solana.MustPublicKeyFromBase58("dbcij3LWUppWqq96dh6gJWwBifmcGfLSB5D4DuSMaqN")

type RpcClient interface {
	GetAccountDataBorshInto(ctx context.Context, account solana.PublicKey, out interface{}) error
}

type Client struct {
	client RpcClient
	logger *zap.Logger
}

func NewClient(
	client RpcClient,
	logger *zap.Logger,
) *Client {
	return &Client{
		client: client,
		logger: logger,
	}
}

func (c *Client) GetPoolConfig(ctx context.Context, account solana.PublicKey) (*PoolConfig, error) {
	var poolConfig PoolConfig
	err := c.client.GetAccountDataBorshInto(ctx, account, &poolConfig)
	if err != nil {
		return nil, err
	}
	return &poolConfig, nil
}

func (c *Client) GetPool(ctx context.Context, account solana.PublicKey) (*Pool, error) {
	var pool Pool
	err := c.client.GetAccountDataBorshInto(ctx, account, &pool)
	if err != nil {
		return nil, err
	}
	return &pool, nil
}

func (c *Client) GetPoolCurveProgress(ctx context.Context, poolAccount solana.PublicKey) (float64, error) {
	pool, err := c.GetPool(ctx, poolAccount)
	if err != nil {
		return 0, err
	}

	config, err := c.GetPoolConfig(ctx, pool.Config)
	if err != nil {
		return 0, err
	}

	return pool.GetMigrationProgress(config.MigrationQuoteThreshold), nil
}

func (c *Client) GetQuotePrice(ctx context.Context, poolAccount solana.PublicKey, tokenBaseDecimals int, tokenQuoteDecimals int) (float64, error) {
	pool, err := c.GetPool(ctx, poolAccount)
	if err != nil {
		return 0, err
	}

	return pool.GetQuotePrice(tokenBaseDecimals, tokenQuoteDecimals), nil
}
