package jobs

import (
	"context"
	"fmt"
	"sync"
	"time"

	"bridgerton.audius.co/birdeye"
	"bridgerton.audius.co/config"
	"bridgerton.audius.co/database"
	"bridgerton.audius.co/logging"
	"bridgerton.audius.co/solana/spl/programs/meteora_dbc"
	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
	"github.com/jackc/pgx/v5"
	"go.uber.org/zap"
)

// Birdeye rate limit is 15 requests per second.
// To be safe, we use 10 requests per second.
const birdeyeDelay = 100 * time.Millisecond

// How many tokens to fetch from the database in one batch.
const tokenPageSize = 1000

type CoinStatsJob struct {
	birdeyeClient *birdeye.Client
	meteoraClient *meteora_dbc.Client
	pool          database.DbPool
	logger        *zap.Logger

	mutex     sync.Mutex
	isRunning bool
}

func NewCoinStatsJob(config config.Config, pool database.DbPool) *CoinStatsJob {
	logger := logging.NewZapLogger(config).Named("CoinStatsJob")
	birdeyeClient := birdeye.New(config.BirdeyeToken)
	rpcClient := rpc.New(config.SolanaConfig.RpcProviders[0])
	meteoraClient := meteora_dbc.NewClient(rpcClient, logger)

	return &CoinStatsJob{
		birdeyeClient: birdeyeClient,
		meteoraClient: meteoraClient,
		logger:        logger,
		pool:          pool,
	}
}

// ScheduleEvery runs the job every `duration` until the context is cancelled.
func (j *CoinStatsJob) ScheduleEvery(ctx context.Context, duration time.Duration) *CoinStatsJob {
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
func (j *CoinStatsJob) Run(ctx context.Context) {
	if err := j.run(ctx); err != nil {
		j.logger.Error("Job run failed", zap.Error(err))
	} else {
		j.logger.Info("Job completed successfully")
	}
}

// For each artist coin in the database, fetches the latest stats from Birdeye and
// updates the artist_coin_stats table. Ensures only one instance runs at a time.
func (j *CoinStatsJob) run(ctx context.Context) error {
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

	count, err := j.getTokenCount(ctx)
	if err != nil {
		return fmt.Errorf("error getting token count: %w", err)
	}

	for offset := 0; offset < count; offset += tokenPageSize {
		batch, err := j.getTokenBatch(ctx, tokenPageSize, offset)
		if err != nil {
			return fmt.Errorf("error getting token batch: %w", err)
		}

		for _, coin := range batch {
			err := j.updateStats(ctx, coin)
			if err != nil {
				j.logger.Error("error updating stats", zap.String("mint", coin.Mint), zap.Error(err))
			}
			if coin.Pool != nil && *coin.Pool != "" {
				err := j.updatePool(ctx, coin)
				if err != nil {
					j.logger.Error("error updating pool", zap.String("mint", coin.Mint), zap.Error(err))
				}
			}

			// Prevent rate limiting on birdeye
			time.Sleep(birdeyeDelay)
		}
		j.logger.Info("Processed batch", zap.Int("offset", offset), zap.Int("batch_size", len(batch)))
	}

	return nil
}

func (j *CoinStatsJob) updateStats(ctx context.Context, coin ArtistCoin) error {
	overview, err := j.birdeyeClient.GetTokenOverview(ctx, coin.Mint, "24h")
	if err != nil {
		return fmt.Errorf("error getting token overview: %w", err)
	}
	err = j.insertArtistCoinStats(ctx, coin.Mint, overview)
	if err != nil {
		return fmt.Errorf("error inserting artist coin stats: %w", err)
	}
	return nil
}

func (j *CoinStatsJob) updatePool(ctx context.Context, coin ArtistCoin) error {
	pool, err := j.meteoraClient.GetPool(ctx, solana.MustPublicKeyFromBase58(*coin.Pool))
	if err != nil {
		return fmt.Errorf("error getting pool: %w", err)
	}

	poolConfig, err := j.meteoraClient.GetPoolConfig(ctx, pool.Config)
	if err != nil {
		return fmt.Errorf("error getting pool config: %w", err)
	}

	price, err := j.meteoraClient.GetQuotePrice(ctx, solana.MustPublicKeyFromBase58(*coin.Pool), int(poolConfig.TokenDecimal), 6)
	if err != nil {
		return fmt.Errorf("error getting quote price: %w", err)
	}

	progress, err := j.meteoraClient.GetPoolCurveProgress(ctx, solana.MustPublicKeyFromBase58(*coin.Pool))
	if err != nil {
		return fmt.Errorf("error getting pool curve progress: %w", err)
	}

	pricesRes, err := j.birdeyeClient.GetPrices(ctx, []string{poolConfig.QuoteMint.String()})
	if err != nil {
		return fmt.Errorf("error getting quote prices: %w", err)
	}

	priceUSD := pricesRes[poolConfig.QuoteMint.String()].Value * price

	err = j.insertPool(ctx, *pool, *poolConfig, price, priceUSD, progress)
	if err != nil {
		return fmt.Errorf("error inserting pool: %w", err)
	}
	return nil
}

func (j *CoinStatsJob) getTokenCount(ctx context.Context) (int, error) {
	var count int
	err := j.pool.QueryRow(ctx, "SELECT COUNT(*) FROM artist_coins").Scan(&count)
	if err != nil {
		return 0, err
	}
	return count, nil
}

type ArtistCoin struct {
	Mint string  `db:"mint"`
	Pool *string `db:"dbc_pool"`
}

func (j *CoinStatsJob) getTokenBatch(ctx context.Context, limit, offset int) ([]ArtistCoin, error) {
	rows, err := j.pool.Query(ctx, "SELECT mint, dbc_pool FROM artist_coins ORDER BY mint LIMIT $1 OFFSET $2", limit, offset)
	if err != nil {
		return nil, err
	}

	coins, err := pgx.CollectRows(rows, pgx.RowToStructByName[ArtistCoin])
	if err != nil {
		return nil, err
	}

	return coins, nil
}

func (j *CoinStatsJob) insertArtistCoinStats(ctx context.Context, mint string, overview *birdeye.TokenOverview) error {
	_, err := j.pool.Exec(ctx, `
        INSERT INTO artist_coin_stats (
            mint, market_cap, fdv, liquidity, last_trade_unix_time, last_trade_human_time, price, history_24h_price,
            price_change_24h_percent, unique_wallet_24h, unique_wallet_history_24h, unique_wallet_24h_change_percent,
            total_supply, circulating_supply, holder, trade_24h, trade_history_24h, trade_24h_change_percent,
            sell_24h, sell_history_24h, sell_24h_change_percent, buy_24h, buy_history_24h, buy_24h_change_percent,
            v_24h, v_24h_usd, v_history_24h, v_history_24h_usd, v_24h_change_percent,
            v_buy_24h, v_buy_24h_usd, v_buy_history_24h, v_buy_history_24h_usd, v_buy_24h_change_percent,
            v_sell_24h, v_sell_24h_usd, v_sell_history_24h, v_sell_history_24h_usd, v_sell_24h_change_percent,
            number_markets, created_at, updated_at
        ) VALUES (
            @mint, @market_cap, @fdv, @liquidity, @last_trade_unix_time, @last_trade_human_time, @price, @history_24h_price,
            @price_change_24h_percent, @unique_wallet_24h, @unique_wallet_history_24h, @unique_wallet_24h_change_percent,
            @total_supply, @circulating_supply, @holder, @trade_24h, @trade_history_24h, @trade_24h_change_percent,
            @sell_24h, @sell_history_24h, @sell_24h_change_percent, @buy_24h, @buy_history_24h, @buy_24h_change_percent,
            @v_24h, @v_24h_usd, @v_history_24h, @v_history_24h_usd, @v_24h_change_percent,
            @v_buy_24h, @v_buy_24h_usd, @v_buy_history_24h, @v_buy_history_24h_usd, @v_buy_24h_change_percent,
            @v_sell_24h, @v_sell_24h_usd, @v_sell_history_24h, @v_sell_history_24h_usd, @v_sell_24h_change_percent,
            @number_markets, NOW(), NOW()
        )
        ON CONFLICT (mint) DO UPDATE SET
            market_cap = EXCLUDED.market_cap,
            fdv = EXCLUDED.fdv,
            liquidity = EXCLUDED.liquidity,
            last_trade_unix_time = EXCLUDED.last_trade_unix_time,
            last_trade_human_time = EXCLUDED.last_trade_human_time,
            price = EXCLUDED.price,
            history_24h_price = EXCLUDED.history_24h_price,
            price_change_24h_percent = EXCLUDED.price_change_24h_percent,
            unique_wallet_24h = EXCLUDED.unique_wallet_24h,
            unique_wallet_history_24h = EXCLUDED.unique_wallet_history_24h,
            unique_wallet_24h_change_percent = EXCLUDED.unique_wallet_24h_change_percent,
            total_supply = EXCLUDED.total_supply,
            circulating_supply = EXCLUDED.circulating_supply,
            holder = EXCLUDED.holder,
            trade_24h = EXCLUDED.trade_24h,
            trade_history_24h = EXCLUDED.trade_history_24h,
            trade_24h_change_percent = EXCLUDED.trade_24h_change_percent,
            sell_24h = EXCLUDED.sell_24h,
            sell_history_24h = EXCLUDED.sell_history_24h,
            sell_24h_change_percent = EXCLUDED.sell_24h_change_percent,
            buy_24h = EXCLUDED.buy_24h,
            buy_history_24h = EXCLUDED.buy_history_24h,
            buy_24h_change_percent = EXCLUDED.buy_24h_change_percent,
            v_24h = EXCLUDED.v_24h,
            v_24h_usd = EXCLUDED.v_24h_usd,
            v_history_24h = EXCLUDED.v_history_24h,
            v_history_24h_usd = EXCLUDED.v_history_24h_usd,
            v_24h_change_percent = EXCLUDED.v_24h_change_percent,
            v_buy_24h = EXCLUDED.v_buy_24h,
            v_buy_24h_usd = EXCLUDED.v_buy_24h_usd,
            v_buy_history_24h = EXCLUDED.v_buy_history_24h,
            v_buy_history_24h_usd = EXCLUDED.v_buy_history_24h_usd,
            v_buy_24h_change_percent = EXCLUDED.v_buy_24h_change_percent,
            v_sell_24h = EXCLUDED.v_sell_24h,
            v_sell_24h_usd = EXCLUDED.v_sell_24h_usd,
            v_sell_history_24h = EXCLUDED.v_sell_history_24h,
            v_sell_history_24h_usd = EXCLUDED.v_sell_history_24h_usd,
            v_sell_24h_change_percent = EXCLUDED.v_sell_24h_change_percent,
            number_markets = EXCLUDED.number_markets,
			updated_at = NOW();
    `, pgx.NamedArgs{
		"mint":                             mint,
		"market_cap":                       overview.MarketCap,
		"fdv":                              overview.FDV,
		"liquidity":                        overview.Liquidity,
		"last_trade_unix_time":             overview.LastTradeUnixTime,
		"last_trade_human_time":            overview.LastTradeHumanTime,
		"price":                            overview.Price,
		"history_24h_price":                overview.History24hPrice,
		"price_change_24h_percent":         overview.PriceChange24hPercent,
		"unique_wallet_24h":                overview.UniqueWallet24h,
		"unique_wallet_history_24h":        overview.UniqueWalletHistory24h,
		"unique_wallet_24h_change_percent": overview.UniqueWallet24hChangePercent,
		"total_supply":                     overview.TotalSupply,
		"circulating_supply":               overview.CirculatingSupply,
		"holder":                           overview.Holder,
		"trade_24h":                        overview.Trade24h,
		"trade_history_24h":                overview.TradeHistory24h,
		"trade_24h_change_percent":         overview.Trade24hChangePercent,
		"sell_24h":                         overview.Sell24h,
		"sell_history_24h":                 overview.SellHistory24h,
		"sell_24h_change_percent":          overview.Sell24hChangePercent,
		"buy_24h":                          overview.Buy24h,
		"buy_history_24h":                  overview.BuyHistory24h,
		"buy_24h_change_percent":           overview.Buy24hChangePercent,
		"v_24h":                            overview.V24h,
		"v_24h_usd":                        overview.V24hUSD,
		"v_history_24h":                    overview.VHistory24h,
		"v_history_24h_usd":                overview.VHistory24hUSD,
		"v_24h_change_percent":             overview.V24hChangePercent,
		"v_buy_24h":                        overview.VBuy24h,
		"v_buy_24h_usd":                    overview.VBuy24hUSD,
		"v_buy_history_24h":                overview.VBuyHistory24h,
		"v_buy_history_24h_usd":            overview.VBuyHistory24hUSD,
		"v_buy_24h_change_percent":         overview.VBuy24hChangePercent,
		"v_sell_24h":                       overview.VSell24h,
		"v_sell_24h_usd":                   overview.VSell24hUSD,
		"v_sell_history_24h":               overview.VSellHistory24h,
		"v_sell_history_24h_usd":           overview.VSellHistory24hUSD,
		"v_sell_24h_change_percent":        overview.VSell24hChangePercent,
		"number_markets":                   overview.NumberMarkets,
	})
	return err
}

func (j *CoinStatsJob) insertPool(
	ctx context.Context,
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
            price = EXCLUDED.price,
			price_usd = EXCLUDED.price_usd,
            curve_progress = EXCLUDED.curve_progress,
            is_migrated = EXCLUDED.is_migrated,
            updated_at = NOW()
    `, pgx.NamedArgs{
		"address":                   pool.Config.String(),
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
		"price":                     price,
		"price_usd":                 priceUSD,
		"curve_progress":            curveProgress,
		"is_migrated":               pool.IsMigrated != 0,
	})
	return err
}
