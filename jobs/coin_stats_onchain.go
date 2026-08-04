package jobs

import (
	"context"
	"fmt"
	"strconv"
	"sync"
	"time"

	"api.audius.co/config"
	"api.audius.co/database"
	"api.audius.co/logging"
	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
	"github.com/jackc/pgx/v5"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
)

// supplyFetchConcurrency bounds concurrent GetTokenSupply RPC calls.
const supplyFetchConcurrency = 8

// supplyFetchTimeout bounds a single GetTokenSupply RPC call.
const supplyFetchTimeout = 10 * time.Second

// TokenSupplyFetcher is the subset of the Solana RPC client the job needs to
// read SPL mint supply. Abstracted so tests can inject a fake without a live RPC.
type TokenSupplyFetcher interface {
	GetTokenSupply(ctx context.Context, tokenMint solana.PublicKey, commitment rpc.CommitmentType) (*rpc.GetTokenSupplyResult, error)
}

// CoinStatsOnchainJob derives artist-coin market stats from on-chain data
// (Meteora DBC/DAMM v2 pools, token-account balances, mint supply) and writes
// them to artist_coin_stats. It is the on-chain replacement for the former
// Birdeye-backed CoinStatsJob.
type CoinStatsOnchainJob struct {
	pool      database.DbPool
	rpcClient TokenSupplyFetcher
	audioMint string
	logger    *zap.Logger

	mutex     sync.Mutex
	isRunning bool
}

func NewCoinStatsOnchainJob(cfg config.Config, pool database.DbPool) *CoinStatsOnchainJob {
	logger := logging.NewZapLogger(cfg).Named("CoinStatsOnchainJob")

	var rpcClient TokenSupplyFetcher
	if len(cfg.SolanaConfig.RpcProviders) > 0 {
		rpcClient = rpc.New(cfg.SolanaConfig.RpcProviders[0])
	}

	return &CoinStatsOnchainJob{
		pool:      pool,
		rpcClient: rpcClient,
		audioMint: cfg.SolanaConfig.MintAudio.String(),
		logger:    logger,
	}
}

// ScheduleEvery runs the job every `duration` until the context is cancelled.
func (j *CoinStatsOnchainJob) ScheduleEvery(ctx context.Context, duration time.Duration) *CoinStatsOnchainJob {
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
func (j *CoinStatsOnchainJob) Run(ctx context.Context) {
	if err := j.run(ctx); err != nil {
		j.logger.Error("Job run failed", zap.Error(err))
	} else {
		j.logger.Info("Job completed successfully")
	}
}

type coinAgg struct {
	Mint      string   `db:"mint"`
	Price     *float64 `db:"price"`
	Holder    int64    `db:"holder"`
	Liquidity float64  `db:"liquidity"`
}

type priceRow struct {
	Mint  string  `db:"mint"`
	Price float64 `db:"price"`
}

func (j *CoinStatsOnchainJob) run(ctx context.Context) error {
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

	now := time.Now()

	// AUDIO USD anchor (maintained by AudioPriceJob). Used to convert all
	// AUDIO-denominated reserves/volume to USD.
	audioPrice, err := j.getAudioPrice(ctx)
	if err != nil {
		return fmt.Errorf("error reading AUDIO price: %w", err)
	}
	if audioPrice == 0 {
		j.logger.Warn("AUDIO price is 0; USD-denominated stats will be 0 this run")
	}

	// 1. Set-based aggregates: on-chain price, holder count, liquidity (TVL).
	aggs, err := j.queryAggregates(ctx, audioPrice)
	if err != nil {
		return fmt.Errorf("error computing aggregates: %w", err)
	}

	// 2. Snapshot current on-chain prices (hourly bin) and read the ~24h-ago price.
	if err := j.snapshotPrices(ctx, now); err != nil {
		return fmt.Errorf("error snapshotting prices: %w", err)
	}
	history24h, err := j.read24hAgoPrices(ctx, now)
	if err != nil {
		return fmt.Errorf("error reading 24h price history: %w", err)
	}

	// 4. Mint supply via RPC (bounded concurrency, skip-on-error).
	mints := make([]string, 0, len(aggs))
	for mint := range aggs {
		mints = append(mints, mint)
	}
	supplies := j.fetchSupplies(ctx, mints)

	// 5. Upsert per coin.
	for mint, agg := range aggs {
		// AUDIO has no artist pool; its on-chain price is NULL. Use the anchor
		// price for market cap and never write NULL over its maintained price.
		effectivePrice := agg.Price
		if mint == j.audioMint {
			ap := audioPrice
			effectivePrice = &ap
		}

		var totalSupply *float64
		if s, ok := supplies[mint]; ok {
			totalSupply = &s
		}

		var marketCap *float64
		if effectivePrice != nil && totalSupply != nil {
			mc := *effectivePrice * *totalSupply
			marketCap = &mc
		}

		var history *float64
		var priceChange *float64
		if h, ok := history24h[mint]; ok && h > 0 {
			history = &h
			if agg.Price != nil {
				pc := ((*agg.Price - h) / h) * 100
				priceChange = &pc
			}
		}

		if err := j.upsertStats(ctx, mint, agg, marketCap, totalSupply, history, priceChange); err != nil {
			j.logger.Error("error upserting onchain stats", zap.String("mint", mint), zap.Error(err))
		}
	}

	// 6. Retention: keep the price-history table small.
	if _, err := j.pool.Exec(ctx,
		`DELETE FROM artist_coin_price_history WHERE timestamp < $1`,
		now.Add(-7*24*time.Hour),
	); err != nil {
		j.logger.Error("error pruning price history", zap.Error(err))
	}

	return nil
}

func (j *CoinStatsOnchainJob) getAudioPrice(ctx context.Context) (float64, error) {
	var price *float64
	err := j.pool.QueryRow(ctx,
		`SELECT price FROM artist_coin_stats WHERE mint = $1`,
		j.audioMint,
	).Scan(&price)
	if err != nil {
		if err == pgx.ErrNoRows {
			return 0, nil
		}
		return 0, err
	}
	if price == nil {
		return 0, nil
	}
	return *price, nil
}

// queryAggregates computes per-coin on-chain price, holder count, and liquidity
// (pool TVL = base-side USD + quote-side USD) for every artist coin.
func (j *CoinStatsOnchainJob) queryAggregates(ctx context.Context, audioPrice float64) (map[string]coinAgg, error) {
	sql := `
		WITH onchain_price AS (
			SELECT mint, COALESCE(damm_v2_price, dbc_price, pools_price_usd) AS price
			FROM artist_coin_prices
		),
		holders AS (
			-- Count token accounts (not distinct owners) to match Birdeye's holder
			-- metric. Distinct-owner would undercount: the claimable-tokens program
			-- authority owns one account per user (the user-bank mechanism), so many
			-- real holders share a single program owner.
			SELECT mint, COUNT(*) AS holder
			FROM sol_token_account_balances
			WHERE balance > 0
			GROUP BY mint
		),
		liq_dbc AS (
			-- Active bonding curve: value both real vault reserves in USD.
			SELECT
				p.base_mint AS mint,
				(p.base_reserve::numeric / POWER(10, ac.decimals)) * COALESCE(op.price, 0)
					+ (p.quote_reserve::numeric / POWER(10, 8)) * @audio_price AS liquidity
			FROM sol_meteora_dbc_pools p
			JOIN artist_coins ac ON ac.mint = p.base_mint
			LEFT JOIN onchain_price op ON op.mint = p.base_mint
			WHERE p.is_migrated = 0
		),
		liq_damm AS (
			-- Migrated pool: value both vault balances in USD (token_a=coin, token_b=AUDIO).
			SELECT
				p.token_a_mint AS mint,
				(COALESCE(ba.balance, 0)::numeric / POWER(10, ac.decimals)) * COALESCE(op.price, 0)
					+ (COALESCE(bb.balance, 0)::numeric / POWER(10, 8)) * @audio_price AS liquidity
			FROM sol_meteora_damm_v2_pools p
			JOIN artist_coins ac ON ac.mint = p.token_a_mint
			LEFT JOIN onchain_price op ON op.mint = p.token_a_mint
			LEFT JOIN sol_token_account_balances ba ON ba.account = p.token_a_vault
			LEFT JOIN sol_token_account_balances bb ON bb.account = p.token_b_vault
		),
		liquidity AS (
			SELECT mint, SUM(liquidity) AS liquidity
			FROM (SELECT * FROM liq_dbc UNION ALL SELECT * FROM liq_damm) x
			GROUP BY mint
		)
		SELECT
			ac.mint,
			op.price::double precision AS price,
			COALESCE(h.holder, 0) AS holder,
			COALESCE(l.liquidity, 0)::double precision AS liquidity
		FROM artist_coins ac
		LEFT JOIN onchain_price op ON op.mint = ac.mint
		LEFT JOIN holders h ON h.mint = ac.mint
		LEFT JOIN liquidity l ON l.mint = ac.mint
	`
	rows, err := j.pool.Query(ctx, sql, pgx.NamedArgs{"audio_price": audioPrice})
	if err != nil {
		return nil, err
	}
	list, err := pgx.CollectRows(rows, pgx.RowToStructByName[coinAgg])
	if err != nil {
		return nil, err
	}
	out := make(map[string]coinAgg, len(list))
	for _, r := range list {
		out[r.Mint] = r
	}
	return out, nil
}

// snapshotPrices records the current on-chain USD price for each coin in an
// hourly bin, used to compute the 24h price change.
func (j *CoinStatsOnchainJob) snapshotPrices(ctx context.Context, now time.Time) error {
	sql := `
		INSERT INTO artist_coin_price_history (mint, timestamp, price)
		SELECT
			mint,
			date_trunc('hour', @now::timestamp),
			COALESCE(damm_v2_price, dbc_price, pools_price_usd)
		FROM artist_coin_prices
		WHERE COALESCE(damm_v2_price, dbc_price, pools_price_usd) IS NOT NULL
		ON CONFLICT (mint, timestamp) DO UPDATE SET price = EXCLUDED.price
	`
	_, err := j.pool.Exec(ctx, sql, pgx.NamedArgs{"now": now})
	return err
}

// read24hAgoPrices returns, per coin, the most recent price snapshot that is at
// least 24h old (and no older than 30h, to avoid stale baselines across gaps).
func (j *CoinStatsOnchainJob) read24hAgoPrices(ctx context.Context, now time.Time) (map[string]float64, error) {
	sql := `
		SELECT DISTINCT ON (mint) mint, price
		FROM artist_coin_price_history
		WHERE timestamp <= @cutoff AND timestamp >= @floor
		ORDER BY mint, timestamp DESC
	`
	rows, err := j.pool.Query(ctx, sql, pgx.NamedArgs{
		"cutoff": now.Add(-24 * time.Hour),
		"floor":  now.Add(-30 * time.Hour),
	})
	if err != nil {
		return nil, err
	}
	list, err := pgx.CollectRows(rows, pgx.RowToStructByName[priceRow])
	if err != nil {
		return nil, err
	}
	out := make(map[string]float64, len(list))
	for _, r := range list {
		out[r.Mint] = r.Price
	}
	return out, nil
}

// fetchSupplies reads SPL mint total supply (human units) per mint over RPC with
// bounded concurrency. Mints that error are omitted (caller keeps last known).
func (j *CoinStatsOnchainJob) fetchSupplies(ctx context.Context, mints []string) map[string]float64 {
	out := make(map[string]float64, len(mints))
	if j.rpcClient == nil {
		j.logger.Warn("no RPC client configured; skipping supply fetch")
		return out
	}

	var mu sync.Mutex
	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(supplyFetchConcurrency)

	for _, mint := range mints {
		g.Go(func() error {
			pubkey, err := solana.PublicKeyFromBase58(mint)
			if err != nil {
				j.logger.Error("invalid mint address", zap.String("mint", mint), zap.Error(err))
				return nil
			}
			callCtx, cancel := context.WithTimeout(gctx, supplyFetchTimeout)
			defer cancel()

			res, err := j.rpcClient.GetTokenSupply(callCtx, pubkey, rpc.CommitmentConfirmed)
			if err != nil {
				j.logger.Warn("failed to fetch token supply", zap.String("mint", mint), zap.Error(err))
				return nil
			}
			supply, ok := uiAmount(res)
			if !ok {
				return nil
			}
			mu.Lock()
			out[mint] = supply
			mu.Unlock()
			return nil
		})
	}
	// Errors are swallowed per-mint; Wait returns nil.
	_ = g.Wait()
	return out
}

// uiAmount extracts the human-unit supply from a GetTokenSupply result.
func uiAmount(res *rpc.GetTokenSupplyResult) (float64, bool) {
	if res == nil || res.Value == nil {
		return 0, false
	}
	if res.Value.UiAmountString != "" {
		if v, err := strconv.ParseFloat(res.Value.UiAmountString, 64); err == nil {
			return v, true
		}
	}
	if res.Value.UiAmount != nil {
		return *res.Value.UiAmount, true
	}
	// Fall back to raw amount / 10^decimals.
	if raw, err := strconv.ParseFloat(res.Value.Amount, 64); err == nil {
		return raw / pow10(int(res.Value.Decimals)), true
	}
	return 0, false
}

func pow10(n int) float64 {
	v := 1.0
	for range n {
		v *= 10
	}
	return v
}

func (j *CoinStatsOnchainJob) upsertStats(
	ctx context.Context,
	mint string,
	agg coinAgg,
	marketCap *float64,
	totalSupply *float64,
	history24h *float64,
	priceChange *float64,
) error {
	sql := `
		INSERT INTO artist_coin_stats (
			mint, price, market_cap, liquidity, holder, total_supply,
			history_24h_price, price_change_24h_percent,
			created_at, updated_at
		) VALUES (
			@mint, @price, @market_cap, @liquidity, @holder, @total_supply,
			@history_24h_price, @price_change_24h_percent,
			NOW(), NOW()
		)
		ON CONFLICT (mint) DO UPDATE SET
			-- Never overwrite a maintained price with NULL. AUDIO's price is set by
			-- AudioPriceJob (from the AUDIO/USDC pool); pool-less coins keep their last price.
			price                    = COALESCE(EXCLUDED.price, artist_coin_stats.price),
			market_cap               = COALESCE(EXCLUDED.market_cap, artist_coin_stats.market_cap),
			liquidity                = EXCLUDED.liquidity,
			holder                   = EXCLUDED.holder,
			total_supply             = COALESCE(EXCLUDED.total_supply, artist_coin_stats.total_supply),
			history_24h_price        = COALESCE(EXCLUDED.history_24h_price, artist_coin_stats.history_24h_price),
			price_change_24h_percent = COALESCE(EXCLUDED.price_change_24h_percent, artist_coin_stats.price_change_24h_percent),
			updated_at               = NOW()
	`
	_, err := j.pool.Exec(ctx, sql, pgx.NamedArgs{
		"mint":                     mint,
		"price":                    agg.Price,
		"market_cap":               marketCap,
		"liquidity":                agg.Liquidity,
		"holder":                   agg.Holder,
		"total_supply":             totalSupply,
		"history_24h_price":        history24h,
		"price_change_24h_percent": priceChange,
	})
	return err
}
