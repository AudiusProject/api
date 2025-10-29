package jobs

import (
	"context"
	"fmt"
	"sync"
	"time"

	"api.audius.co/config"
	"api.audius.co/database"
	"api.audius.co/logging"
	"github.com/jackc/pgx/v5"
	"go.uber.org/zap"
)

type BalanceHistoryJob struct {
	pool     database.DbPool
	logger   *zap.Logger
	usdcMint string

	mutex     sync.Mutex
	isRunning bool
}

func NewBalanceHistoryJob(cfg config.Config, pool database.DbPool) *BalanceHistoryJob {
	logger := logging.NewZapLogger(cfg).Named("BalanceHistoryJob")

	return &BalanceHistoryJob{
		logger:   logger,
		pool:     pool,
		usdcMint: cfg.SolanaConfig.MintUSDC.String(),
	}
}

// ScheduleEvery runs the job every `duration` until the context is cancelled.
func (j *BalanceHistoryJob) ScheduleEvery(ctx context.Context, duration time.Duration) *BalanceHistoryJob {
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
func (j *BalanceHistoryJob) Run(ctx context.Context) {
	if err := j.run(ctx); err != nil {
		j.logger.Error("Job run failed", zap.Error(err))
	} else {
		j.logger.Info("Job completed successfully")
	}
}

// Records hourly balance snapshots for all users with non-zero balances.
// Stores one row per (user_id, mint, timestamp) combination.
// Ensures only one instance runs at a time.
func (j *BalanceHistoryJob) run(ctx context.Context) error {
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

	// Truncate to the current hour
	now := time.Now()
	timestamp := time.Date(now.Year(), now.Month(), now.Day(), now.Hour(), 0, 0, 0, now.Location())

	j.logger.Info("Recording balance history", zap.Time("timestamp", timestamp))

	sql := `
		-- Insert balance history for all users and mints with non-zero balances
		INSERT INTO user_balance_history (user_id, mint, timestamp, balance, balance_usd, created_at)
		-- Artist coin balances
		SELECT
			sub.user_id,
			sub.mint,
			date_trunc('hour', @timestamp::timestamp) AS timestamp,
			sub.balance,
			-- Calculate USD value: (balance / 10^decimals) * price
			(sub.balance::NUMERIC / POWER(10, ac.decimals)) * COALESCE(acp.price, 0) AS balance_usd,
			NOW() AS created_at
		FROM sol_user_balances sub
		JOIN artist_coins ac ON ac.mint = sub.mint
		LEFT JOIN artist_coin_prices acp ON acp.mint = sub.mint
		WHERE sub.balance > 0
			AND COALESCE(acp.price, 0) > 0
		UNION ALL
		-- USDC balances (USDC has 6 decimals and fixed price of $1.00)
		SELECT
			sub.user_id,
			sub.mint,
			date_trunc('hour', @timestamp::timestamp) AS timestamp,
			sub.balance,
			-- USDC price is $1.00, so balance_usd = balance / 10^6
			(sub.balance::NUMERIC / POWER(10, 6)) AS balance_usd,
			NOW() AS created_at
		FROM sol_user_balances sub
		WHERE sub.mint = @usdc_mint
			AND sub.balance > 0
		ON CONFLICT (user_id, timestamp, mint) 
		DO UPDATE SET
			balance = EXCLUDED.balance,
			balance_usd = EXCLUDED.balance_usd,
			created_at = NOW()
	;`

	result, err := j.pool.Exec(ctx, sql, pgx.NamedArgs{
		"timestamp": timestamp,
		"usdc_mint": j.usdcMint,
	})
	if err != nil {
		return fmt.Errorf("error recording balance history: %w", err)
	}

	j.logger.Info("Balance history recorded",
		zap.Int64("rows_affected", result.RowsAffected()),
		zap.Time("timestamp", timestamp),
	)

	return nil
}

// RecordUserBalanceHistory records balance history for a specific user
// Useful for on-demand updates after balance changes
func RecordUserBalanceHistory(ctx context.Context, db database.DBTX, userId int32, usdcMint string) error {
	// Truncate to the current hour
	now := time.Now()
	timestamp := time.Date(now.Year(), now.Month(), now.Day(), now.Hour(), 0, 0, 0, now.Location())

	sql := `
		INSERT INTO user_balance_history (user_id, mint, timestamp, balance, balance_usd, created_at)
		-- Artist coin balances
		SELECT
			sub.user_id,
			sub.mint,
			date_trunc('hour', @timestamp::timestamp) AS timestamp,
			sub.balance,
			-- Calculate USD value: (balance / 10^decimals) * price
			(sub.balance::NUMERIC / POWER(10, ac.decimals)) * COALESCE(acp.price, 0) AS balance_usd,
			NOW() AS created_at
		FROM sol_user_balances sub
		JOIN artist_coins ac ON ac.mint = sub.mint
		LEFT JOIN artist_coin_prices acp ON acp.mint = sub.mint
		WHERE sub.user_id = @user_id
			AND sub.balance > 0
			AND COALESCE(acp.price, 0) > 0
		UNION ALL
		-- USDC balances (USDC has 6 decimals and fixed price of $1.00)
		SELECT
			sub.user_id,
			sub.mint,
			date_trunc('hour', @timestamp::timestamp) AS timestamp,
			sub.balance,
			-- USDC price is $1.00, so balance_usd = balance / 10^6
			(sub.balance::NUMERIC / POWER(10, 6)) AS balance_usd,
			NOW() AS created_at
		FROM sol_user_balances sub
		WHERE sub.user_id = @user_id
			AND sub.mint = @usdc_mint
			AND sub.balance > 0
		ON CONFLICT (user_id, timestamp, mint) 
		DO UPDATE SET
			balance = EXCLUDED.balance,
			balance_usd = EXCLUDED.balance_usd,
			created_at = NOW()
	;`

	_, err := db.Exec(ctx, sql, pgx.NamedArgs{
		"user_id":   userId,
		"timestamp": timestamp,
		"usdc_mint": usdcMint,
	})
	if err != nil {
		return fmt.Errorf("error recording balance history for user %d: %w", userId, err)
	}

	return nil
}
