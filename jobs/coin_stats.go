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
	pool          database.DbPool
	logger        *zap.Logger

	mutex     sync.Mutex
	isRunning bool
}

func NewCoinStatsJob(config config.Config, pool database.DbPool) *CoinStatsJob {
	logger := logging.NewZapLogger(config).Named("CoinStatsJob")
	birdeyeClient := birdeye.New(config.BirdeyeToken)

	return &CoinStatsJob{
		birdeyeClient: birdeyeClient,
		logger:        logger,
		pool:          pool,
	}
}

// ScheduleEvery runs the job every `duration` until the context is cancelled.
func (j *CoinStatsJob) ScheduleEvery(ctx context.Context, duration time.Duration) error {
	ticker := time.NewTicker(duration)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			j.logger.Info("Job started")
			if err := j.run(ctx); err != nil {
				j.logger.Error("Job run failed", zap.Error(err))
			} else {
				j.logger.Info("Job completed successfully")
			}
		case <-ctx.Done():
			j.logger.Info("Job shutting down")
			return nil
		}
	}
}

// run executes the job once. Ensures only one instance runs at a time.
// For each artist coin in the database, fetches the latest stats from Birdeye and
// updates the artist_coin_stats table.
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

		for _, mint := range batch {
			overview, err := j.birdeyeClient.GetTokenOverview(ctx, mint, "24h")
			if err != nil {
				return fmt.Errorf("error getting token overview for mint %s: %w", mint, err)
			}
			err = j.insertArtistCoinStats(ctx, mint, overview)
			if err != nil {
				return fmt.Errorf("error inserting artist coin stats for mint %s: %w", mint, err)
			}

			// Prevent rate limiting
			time.Sleep(birdeyeDelay)
		}
		j.logger.Info("Processed batch", zap.Int("offset", offset), zap.Int("batch_size", len(batch)))
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

func (j *CoinStatsJob) getTokenBatch(ctx context.Context, limit, offset int) ([]string, error) {
	rows, err := j.pool.Query(ctx, "SELECT mint FROM artist_coins ORDER BY mint LIMIT $1 OFFSET $2", limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	mints := make([]string, 0, limit)
	for rows.Next() {
		var mint string
		if err := rows.Scan(&mint); err != nil {
			return nil, err
		}
		mints = append(mints, mint)
	}
	return mints, nil
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
