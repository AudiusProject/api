package jobs

import (
	"context"
	"fmt"
	"sync"
	"time"

	"api.audius.co/config"
	"api.audius.co/database"
	"api.audius.co/logging"
	"api.audius.co/solana/spl/programs/meteora_dbc"
	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
	"github.com/jackc/pgx/v5"
	"github.com/krazyTry/meteora-go/dbc"
	"go.uber.org/zap"
)

const AUDIO_DECIMALS = 8

type CoinDBCJob struct {
	meteoraClient *meteora_dbc.Client
	dbc           *dbc.DBC
	pool          database.DbPool
	logger        *zap.Logger

	mutex     sync.Mutex
	isRunning bool
}

func NewCoinDBCJob(config config.Config, pool database.DbPool) *CoinDBCJob {
	logger := logging.NewZapLogger(config).Named("CoinDbcJob")
	rpcClient := rpc.New(config.SolanaConfig.RpcProviders[0])
	meteoraClient := meteora_dbc.NewClient(rpcClient, logger)
	dbc := dbc.NewDBC(rpcClient)

	return &CoinDBCJob{
		meteoraClient: meteoraClient,
		logger:        logger,
		pool:          pool,
		dbc:           dbc,
	}
}

// ScheduleEvery runs the job every `duration` until the context is cancelled.
func (j *CoinDBCJob) ScheduleEvery(ctx context.Context, duration time.Duration) *CoinDBCJob {
	go func() {
		ticker := time.NewTicker(duration)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				j.logger.Info("Job started")
				j.Run(ctx)
			case <-ctx.Done():
				j.logger.Info("Job shutting down")
				return
			}
		}
	}()
	return j
}

// Run executes the job once
func (j *CoinDBCJob) Run(ctx context.Context) {
	if err := j.run(ctx); err != nil {
		j.logger.Error("Job run failed", zap.Error(err))
	} else {
		j.logger.Info("Job completed successfully")
	}
}

// For each artist coin in the database, fetches the latest stats from the Meteora DBC
// updates the artist_coin_pools table. Ensures only one instance runs at a time.
func (j *CoinDBCJob) run(ctx context.Context) error {
	j.mutex.Lock()
	if j.isRunning {
		j.mutex.Unlock()
		return fmt.Errorf("job is already running")
	}
	j.isRunning = true
	j.mutex.Unlock()
	defer func() {
		j.mutex.Lock()
		j.isRunning = false
		j.mutex.Unlock()
	}()

	count, err := GetTokenCount(ctx, j.pool)
	if err != nil {
		return fmt.Errorf("error getting token count: %w", err)
	}

	for offset := 0; offset < count; offset += tokenPageSize {
		batch, err := GetTokenBatch(ctx, j.pool, tokenPageSize, offset)
		if err != nil {
			return fmt.Errorf("error getting token batch: %w", err)
		}

		for _, coin := range batch {
			baseMint := solana.MustPublicKeyFromBase58(coin.Mint)
			pool, err := j.dbc.GetPoolByBaseMint(ctx, baseMint)
			if err != nil {
				j.logger.Error("pool lookup failed", zap.String("mint", coin.Mint), zap.Error(err))
				continue
			}
			if pool == nil {
				j.logger.Debug("No pool found for mint", zap.String("mint", coin.Mint))
				continue
			}
			poolPubkey := pool.Address
			if err := j.updatePool(ctx, poolPubkey); err != nil {
				j.logger.Error("error updating pool", zap.String("mint", coin.Mint), zap.Error(err))
			}
		}
		j.logger.Info("Processed batch", zap.Int("offset", offset), zap.Int("batch_size", len(batch)))
	}

	return nil
}

func (j *CoinDBCJob) updatePool(ctx context.Context, poolPubkey solana.PublicKey) error {
	pool, err := j.meteoraClient.GetPool(ctx, poolPubkey)
	if err != nil {
		return fmt.Errorf("error getting pool: %w", err)
	}

	poolConfig, err := j.meteoraClient.GetPoolConfig(ctx, pool.Config)
	if err != nil {
		return fmt.Errorf("error getting pool config: %w", err)
	}

	price, err := j.meteoraClient.GetQuotePrice(ctx, poolPubkey, int(poolConfig.TokenDecimal), AUDIO_DECIMALS)
	if err != nil {
		return fmt.Errorf("error getting quote price: %w", err)
	}

	progress, err := j.meteoraClient.GetPoolCurveProgress(ctx, poolPubkey)
	if err != nil {
		return fmt.Errorf("error getting pool curve progress: %w", err)
	}

	// Fetch USD price for quote mint from database; default to 0 if not found
	var quoteUSD float64
	if err := j.pool.QueryRow(ctx, "SELECT price FROM artist_coin_stats WHERE mint = $1", poolConfig.QuoteMint.String()).Scan(&quoteUSD); err != nil {
		j.logger.Error("error querying quote price; defaulting to 0", zap.String("quote_mint", poolConfig.QuoteMint.String()), zap.Error(err))
		quoteUSD = 0
	}
	priceUSD := quoteUSD * price

	err = j.insertPool(ctx, poolPubkey, *pool, *poolConfig, price, priceUSD, progress)
	if err != nil {
		j.logger.Error("error inserting pool", zap.Error(err))
		return fmt.Errorf("error inserting pool: %w", err)
	}
	return nil
}

func (j *CoinDBCJob) insertPool(
	ctx context.Context,
	poolAddress solana.PublicKey,
	pool meteora_dbc.Pool,
	poolConfig meteora_dbc.PoolConfig,
	price float64,
	priceUSD float64,
	curveProgress float64,
) error {
	_, err := j.pool.Exec(ctx, `
        INSERT INTO artist_coin_pools (
            address,
            base_mint,
            quote_mint,
            token_decimals,
            base_reserve,
            quote_reserve,
            migration_base_threshold,
            migration_quote_threshold,
            protocol_quote_fee,
            partner_quote_fee,
            creator_base_fee,
            creator_quote_fee,
            total_trading_quote_fee,
            price,
			price_usd,
            curve_progress,
            is_migrated,
            updated_at
        ) VALUES (
            @address,
            @base_mint,
            @quote_mint,
            @token_decimals,
            @base_reserve,
            @quote_reserve,
            @migration_base_threshold,
            @migration_quote_threshold,
            @protocol_quote_fee,
            @partner_quote_fee,
            @creator_base_fee,
            @creator_quote_fee,
            @total_trading_quote_fee,
            @price,
			@price_usd,
            @curve_progress,
            @is_migrated,
            NOW()
        )
        ON CONFLICT (address) DO UPDATE SET
            base_mint = EXCLUDED.base_mint,
            quote_mint = EXCLUDED.quote_mint,
            token_decimals = EXCLUDED.token_decimals,
            base_reserve = EXCLUDED.base_reserve,
            quote_reserve = EXCLUDED.quote_reserve,
            migration_quote_threshold = EXCLUDED.migration_quote_threshold,
            migration_base_threshold = EXCLUDED.migration_base_threshold,
            protocol_quote_fee = EXCLUDED.protocol_quote_fee,
            partner_quote_fee = EXCLUDED.partner_quote_fee,
            creator_base_fee = EXCLUDED.creator_base_fee,
            creator_quote_fee = EXCLUDED.creator_quote_fee,
            total_trading_quote_fee = EXCLUDED.total_trading_quote_fee,
            price = EXCLUDED.price,
			price_usd = EXCLUDED.price_usd,
            curve_progress = EXCLUDED.curve_progress,
            is_migrated = EXCLUDED.is_migrated,
            updated_at = NOW()
    `, pgx.NamedArgs{
		"address":                   poolAddress.String(),
		"base_mint":                 pool.BaseMint.String(),
		"quote_mint":                poolConfig.QuoteMint.String(),
		"token_decimals":            int(poolConfig.TokenDecimal),
		"base_reserve":              pool.BaseReserve,
		"quote_reserve":             pool.QuoteReserve,
		"migration_quote_threshold": poolConfig.MigrationQuoteThreshold,
		"migration_base_threshold":  poolConfig.MigrationBaseThreshold,
		"protocol_quote_fee":        pool.ProtocolQuoteFee,
		"partner_quote_fee":         pool.PartnerQuoteFee,
		"creator_base_fee":          pool.CreatorBaseFee,
		"creator_quote_fee":         pool.CreatorQuoteFee,
		"total_trading_quote_fee":   pool.Metrics.TotalTradingQuoteFee,
		"price":                     price,
		"price_usd":                 priceUSD,
		"curve_progress":            curveProgress,
		"is_migrated":               pool.IsMigrated != 0,
	})
	return err
}
