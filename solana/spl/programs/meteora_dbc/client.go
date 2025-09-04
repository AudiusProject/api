package meteora_dbc

import (
	"context"
	"math/big"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
	"go.uber.org/zap"
)

type Client struct {
	client *rpc.Client
	logger *zap.Logger
}

func NewClient(
	client *rpc.Client,
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

	quoteReserve := new(big.Int).SetUint64(pool.QuoteReserve)
	migrationQuoteThreshold := new(big.Int).SetUint64(config.MigrationQuoteThreshold)
	quotient := new(big.Rat).SetFrac(quoteReserve, migrationQuoteThreshold)
	progress, _ := quotient.Float64()

	return progress, nil
}

func (c *Client) GetQuotePrice(ctx context.Context, poolAccount solana.PublicKey, tokenBaseDecimals int, tokenQuoteDecimals int) (float64, error) {
	pool, err := c.GetPool(ctx, poolAccount)
	if err != nil {
		return 0, err
	}

	sqrtPrice := pool.SqrtPrice.BigInt()
	sqrtPriceSquared := new(big.Int).Mul(sqrtPrice, sqrtPrice)
	decimalsFactor := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(tokenBaseDecimals-tokenQuoteDecimals)), nil)

	numerator := new(big.Int).Mul(sqrtPriceSquared, decimalsFactor)
	divisor := new(big.Int).Exp(big.NewInt(2), big.NewInt(128), nil)
	quotient := new(big.Rat).SetFrac(numerator, divisor)

	price, _ := quotient.Float64()

	return price, nil
}
