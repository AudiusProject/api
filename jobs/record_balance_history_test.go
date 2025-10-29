package jobs

import (
	"context"
	"testing"
	"time"

	"api.audius.co/config"
	"api.audius.co/database"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestConfig creates a test config with properly initialized SolanaConfig
func newTestConfig() config.Config {
	cfg := config.Config{Env: "test"}
	cfg.SolanaConfig = config.NewSolanaConfig()
	return cfg
}

func TestRecordBalanceHistory(t *testing.T) {
	pool := database.CreateTestDatabase(t, "test_api")
	defer pool.Close()

	ctx := context.Background()
	cfg := newTestConfig()
	usdcMint := cfg.SolanaConfig.MintUSDC.String()

	// Create test data
	fixtures := database.FixtureMap{
		"users": {
			{"user_id": 1, "wallet": "0x1234567890123456789012345678901234567890"},
			{"user_id": 2, "wallet": "0x0987654321098765432109876543210987654321"},
		},
		"artist_coins": {
			{
				"ticker":   "COIN1",
				"decimals": 6,
				"user_id":  1,
				"mint":     "mint1",
			},
			{
				"ticker":   "COIN2",
				"decimals": 8,
				"user_id":  2,
				"mint":     "mint2",
			},
			{
				"ticker":   "AUDIO",
				"decimals": 8,
				"user_id":  1,
				"mint":     "EhhTKczWMGQt46ynNeRX1WfeagwwJd7ufHvCDjRxjo5Q",
			},
		},
		"artist_coin_stats": {
			{
				"mint":  "mint1",
				"price": 0.5, // $0.50 per COIN1
			},
			{
				"mint":  "mint2",
				"price": 1.0, // $1.00 per COIN2
			},
			{
				"mint":  "EhhTKczWMGQt46ynNeRX1WfeagwwJd7ufHvCDjRxjo5Q",
				"price": 0.25, // $0.25 per AUDIO
			},
		},
		"sol_user_balances": {
			// User 1: 100 COIN1
			{
				"user_id": 1,
				"mint":    "mint1",
				"balance": 100000000, // 100 tokens (decimals 6)
			},
			// User 1: 50 AUDIO
			{
				"user_id": 1,
				"mint":    "EhhTKczWMGQt46ynNeRX1WfeagwwJd7ufHvCDjRxjo5Q",
				"balance": 5000000000, // 50 tokens (decimals 8)
			},
			// User 1: 100 USDC
			{
				"user_id": 1,
				"mint":    usdcMint,
				"balance": 100000000, // 100 USDC (decimals 6)
			},
			// User 2: 200 COIN2
			{
				"user_id": 2,
				"mint":    "mint2",
				"balance": 20000000000, // 200 tokens (decimals 8)
			},
		},
	}

	database.Seed(pool, fixtures)

	// Run the job
	job := NewBalanceHistoryJob(cfg, pool)
	err := job.run(ctx)
	require.NoError(t, err)

	// Verify data was recorded
	var count int
	err = pool.QueryRow(ctx, "SELECT COUNT(*) FROM user_balance_history").Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 4, count, "Should have 4 rows (3 for user 1, 1 for user 2)")

	// Verify user 1's balances
	rows, err := pool.Query(ctx, `
		SELECT mint, balance, balance_usd 
		FROM user_balance_history 
		WHERE user_id = 1 
		ORDER BY mint
	`)
	require.NoError(t, err)
	defer rows.Close()

	var results []struct {
		mint       string
		balance    int64
		balanceUsd float64
	}

	for rows.Next() {
		var r struct {
			mint       string
			balance    int64
			balanceUsd float64
		}
		err := rows.Scan(&r.mint, &r.balance, &r.balanceUsd)
		require.NoError(t, err)
		results = append(results, r)
	}

	require.Equal(t, 3, len(results))

	// USDC should be first alphabetically (starts with "2")
	assert.Equal(t, usdcMint, results[0].mint)
	assert.Equal(t, int64(100000000), results[0].balance)
	assert.InDelta(t, 100.0, results[0].balanceUsd, 0.01) // 100 USDC * $1.00 = $100.00

	// AUDIO should be second (starts with "E")
	assert.Equal(t, "EhhTKczWMGQt46ynNeRX1WfeagwwJd7ufHvCDjRxjo5Q", results[1].mint)
	assert.Equal(t, int64(5000000000), results[1].balance)
	assert.InDelta(t, 12.5, results[1].balanceUsd, 0.01) // 50 * 0.25 = $12.50

	// COIN1 should be third (starts with "m")
	assert.Equal(t, "mint1", results[2].mint)
	assert.Equal(t, int64(100000000), results[2].balance)
	assert.InDelta(t, 50.0, results[2].balanceUsd, 0.01) // 100 * 0.50 = $50.00
}

func TestRecordBalanceHistoryTimestampTruncation(t *testing.T) {
	pool := database.CreateTestDatabase(t, "test_api")
	defer pool.Close()

	ctx := context.Background()

	fixtures := database.FixtureMap{
		"users": {
			{"user_id": 1, "wallet": "0x1234567890123456789012345678901234567890"},
		},
		"artist_coins": {
			{"ticker": "COIN1", "decimals": 6, "user_id": 1, "mint": "mint1"},
		},
		"artist_coin_stats": {
			{"mint": "mint1", "price": 1.0},
		},
		"sol_user_balances": {
			{"user_id": 1, "mint": "mint1", "balance": 100000000},
		},
	}

	database.Seed(pool, fixtures)

	// Run job at a specific time (e.g., 2025-01-15 14:37:42)
	// Should truncate to 2025-01-15 14:00:00
	job := NewBalanceHistoryJob(newTestConfig(), pool)
	err := job.run(ctx)
	require.NoError(t, err)

	// Verify timestamp is truncated to the hour
	var timestamp time.Time
	err = pool.QueryRow(ctx, "SELECT timestamp FROM user_balance_history WHERE user_id = 1 LIMIT 1").Scan(&timestamp)
	require.NoError(t, err)

	// Timestamp should have 0 minutes, 0 seconds, 0 nanoseconds
	assert.Equal(t, 0, timestamp.Minute(), "Minutes should be 0")
	assert.Equal(t, 0, timestamp.Second(), "Seconds should be 0")
	assert.Equal(t, 0, timestamp.Nanosecond(), "Nanoseconds should be 0")
}

func TestRecordBalanceHistoryConflictHandling(t *testing.T) {
	pool := database.CreateTestDatabase(t, "test_api")
	defer pool.Close()

	ctx := context.Background()

	fixtures := database.FixtureMap{
		"users": {
			{"user_id": 1, "wallet": "0x1234567890123456789012345678901234567890"},
		},
		"artist_coins": {
			{"ticker": "COIN1", "decimals": 6, "user_id": 1, "mint": "mint1"},
		},
		"artist_coin_stats": {
			{"mint": "mint1", "price": 1.0},
		},
		"sol_user_balances": {
			{"user_id": 1, "mint": "mint1", "balance": 100000000}, // 100 tokens
		},
	}

	database.Seed(pool, fixtures)

	// Run job first time
	job := NewBalanceHistoryJob(newTestConfig(), pool)
	err := job.run(ctx)
	require.NoError(t, err)

	// Verify initial data
	var firstBalanceUsd float64
	err = pool.QueryRow(ctx, "SELECT balance_usd FROM user_balance_history WHERE user_id = 1 LIMIT 1").Scan(&firstBalanceUsd)
	require.NoError(t, err)
	assert.InDelta(t, 100.0, firstBalanceUsd, 0.01)

	// Update user's balance
	_, err = pool.Exec(ctx, "UPDATE sol_user_balances SET balance = 200000000 WHERE user_id = 1 AND mint = 'mint1'")
	require.NoError(t, err)

	// Run job again (same hour) - should UPDATE existing row
	err = job.run(ctx)
	require.NoError(t, err)

	// Verify data was updated
	var updatedBalanceUsd float64
	err = pool.QueryRow(ctx, "SELECT balance_usd FROM user_balance_history WHERE user_id = 1 LIMIT 1").Scan(&updatedBalanceUsd)
	require.NoError(t, err)
	assert.InDelta(t, 200.0, updatedBalanceUsd, 0.01) // Should be updated to 200 tokens * $1

	// Verify only 1 row exists (updated, not duplicated)
	var count int
	err = pool.QueryRow(ctx, "SELECT COUNT(*) FROM user_balance_history WHERE user_id = 1").Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 1, count, "Should only have 1 row (updated, not duplicated)")
}

func TestRecordBalanceHistoryZeroBalance(t *testing.T) {
	pool := database.CreateTestDatabase(t, "test_api")
	defer pool.Close()

	ctx := context.Background()

	fixtures := database.FixtureMap{
		"users": {
			{"user_id": 1, "wallet": "0x1234567890123456789012345678901234567890"},
		},
		"artist_coins": {
			{"ticker": "COIN1", "decimals": 6, "user_id": 1, "mint": "mint1"},
		},
		"artist_coin_stats": {
			{"mint": "mint1", "price": 1.0},
		},
		"sol_user_balances": {
			// User has zero balance - should not be recorded
			{"user_id": 1, "mint": "mint1", "balance": 0},
		},
	}

	database.Seed(pool, fixtures)

	// Run job
	job := NewBalanceHistoryJob(newTestConfig(), pool)
	err := job.run(ctx)
	require.NoError(t, err)

	// Verify no data was recorded for zero balance
	var count int
	err = pool.QueryRow(ctx, "SELECT COUNT(*) FROM user_balance_history WHERE user_id = 1").Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 0, count, "Should not record zero balances")
}

func TestRecordBalanceHistoryNullPrice(t *testing.T) {
	pool := database.CreateTestDatabase(t, "test_api")
	defer pool.Close()

	ctx := context.Background()

	fixtures := database.FixtureMap{
		"users": {
			{"user_id": 1, "wallet": "0x1234567890123456789012345678901234567890"},
		},
		"artist_coins": {
			{"ticker": "COIN1", "decimals": 6, "user_id": 1, "mint": "mint1"},
		},
		"artist_coin_stats": {
			// Price is NULL (coin not traded yet)
			{"mint": "mint1", "price": nil},
		},
		"sol_user_balances": {
			{"user_id": 1, "mint": "mint1", "balance": 100000000},
		},
	}

	database.Seed(pool, fixtures)

	// Run job
	job := NewBalanceHistoryJob(newTestConfig(), pool)
	err := job.run(ctx)
	require.NoError(t, err)

	// Verify no data was recorded when price is null/0
	var count int
	err = pool.QueryRow(ctx, "SELECT COUNT(*) FROM user_balance_history WHERE user_id = 1").Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 0, count, "Should not record balances with null/zero price")
}

func TestRecordUserBalanceHistory(t *testing.T) {
	pool := database.CreateTestDatabase(t, "test_api")
	defer pool.Close()

	ctx := context.Background()

	fixtures := database.FixtureMap{
		"users": {
			{"user_id": 1, "wallet": "0x1234567890123456789012345678901234567890"},
			{"user_id": 2, "wallet": "0x0987654321098765432109876543210987654321"},
		},
		"artist_coins": {
			{"ticker": "COIN1", "decimals": 6, "user_id": 1, "mint": "mint1"},
		},
		"artist_coin_stats": {
			{"mint": "mint1", "price": 2.0},
		},
		"sol_user_balances": {
			{"user_id": 1, "mint": "mint1", "balance": 50000000},  // 50 tokens
			{"user_id": 2, "mint": "mint1", "balance": 100000000}, // 100 tokens
		},
	}

	database.Seed(pool, fixtures)

	// Record balance for user 1 only
	// Use a test USDC mint address (not used in this test since no USDC balances)
	err := RecordUserBalanceHistory(ctx, pool, 1, "26Q7gP8UfkDzi7GMFEQxTJaNJ8D2ybCUjex58M5MLu8y")
	require.NoError(t, err)

	// Verify only user 1's balance was recorded
	var count int
	err = pool.QueryRow(ctx, "SELECT COUNT(*) FROM user_balance_history WHERE user_id = 1").Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 1, count, "Should have recorded user 1's balance")

	// Verify user 2's balance was NOT recorded
	err = pool.QueryRow(ctx, "SELECT COUNT(*) FROM user_balance_history WHERE user_id = 2").Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 0, count, "Should not have recorded user 2's balance")

	// Verify the value is correct
	var balanceUsd float64
	err = pool.QueryRow(ctx, "SELECT balance_usd FROM user_balance_history WHERE user_id = 1").Scan(&balanceUsd)
	require.NoError(t, err)
	assert.InDelta(t, 100.0, balanceUsd, 0.01) // 50 tokens * $2.00 = $100
}

func TestRecordBalanceHistoryMultipleMints(t *testing.T) {
	pool := database.CreateTestDatabase(t, "test_api")
	defer pool.Close()

	ctx := context.Background()

	fixtures := database.FixtureMap{
		"users": {
			{"user_id": 1, "wallet": "0x1234567890123456789012345678901234567890"},
		},
		"artist_coins": {
			{"ticker": "COIN1", "decimals": 6, "user_id": 1, "mint": "mint1"},
			{"ticker": "COIN2", "decimals": 8, "user_id": 1, "mint": "mint2"},
			{"ticker": "COIN3", "decimals": 6, "user_id": 1, "mint": "mint3"},
		},
		"artist_coin_stats": {
			{"mint": "mint1", "price": 0.5},
			{"mint": "mint2", "price": 1.0},
			{"mint": "mint3", "price": 2.0},
		},
		"sol_user_balances": {
			{"user_id": 1, "mint": "mint1", "balance": 100000000},  // 100 * 0.5 = $50
			{"user_id": 1, "mint": "mint2", "balance": 5000000000}, // 50 * 1.0 = $50
			{"user_id": 1, "mint": "mint3", "balance": 25000000},   // 25 * 2.0 = $50
		},
	}

	database.Seed(pool, fixtures)

	// Run job
	job := NewBalanceHistoryJob(newTestConfig(), pool)
	err := job.run(ctx)
	require.NoError(t, err)

	// Verify 3 separate rows were created (one per mint)
	var count int
	err = pool.QueryRow(ctx, "SELECT COUNT(*) FROM user_balance_history WHERE user_id = 1").Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 3, count, "Should have 3 rows (one per mint)")

	// Verify total balance is correct (sum across mints)
	var totalBalanceUsd float64
	err = pool.QueryRow(ctx, "SELECT SUM(balance_usd) FROM user_balance_history WHERE user_id = 1").Scan(&totalBalanceUsd)
	require.NoError(t, err)
	assert.InDelta(t, 150.0, totalBalanceUsd, 0.01) // 50 + 50 + 50 = $150
}

func TestRecordBalanceHistoryConcurrentRuns(t *testing.T) {
	pool := database.CreateTestDatabase(t, "test_api")
	defer pool.Close()

	ctx := context.Background()

	fixtures := database.FixtureMap{
		"users": {
			{"user_id": 1, "wallet": "0x1234567890123456789012345678901234567890"},
		},
		"artist_coins": {
			{"ticker": "COIN1", "decimals": 6, "user_id": 1, "mint": "mint1"},
		},
		"artist_coin_stats": {
			{"mint": "mint1", "price": 1.0},
		},
		"sol_user_balances": {
			{"user_id": 1, "mint": "mint1", "balance": 100000000},
		},
	}

	database.Seed(pool, fixtures)

	job := NewBalanceHistoryJob(newTestConfig(), pool)

	// First run should succeed
	err := job.run(ctx)
	require.NoError(t, err)

	// Simulate job still running by manually setting isRunning
	job.mutex.Lock()
	job.isRunning = true
	job.mutex.Unlock()

	// Second run should fail with "already running" error
	err = job.run(ctx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already running")

	// Reset for cleanup
	job.mutex.Lock()
	job.isRunning = false
	job.mutex.Unlock()
}

func TestRecordBalanceHistoryNoUsers(t *testing.T) {
	pool := database.CreateTestDatabase(t, "test_api")
	defer pool.Close()

	ctx := context.Background()

	// No users with balances
	fixtures := database.FixtureMap{
		"users": {
			{"user_id": 1, "wallet": "0x1234567890123456789012345678901234567890"},
		},
	}

	database.Seed(pool, fixtures)

	// Run job
	job := NewBalanceHistoryJob(newTestConfig(), pool)
	err := job.run(ctx)
	require.NoError(t, err) // Should not error, just record 0 rows

	// Verify no data was recorded
	var count int
	err = pool.QueryRow(ctx, "SELECT COUNT(*) FROM user_balance_history").Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 0, count)
}

func TestRecordBalanceHistoryDecimalPrecision(t *testing.T) {
	pool := database.CreateTestDatabase(t, "test_api")
	defer pool.Close()

	ctx := context.Background()

	fixtures := database.FixtureMap{
		"users": {
			{"user_id": 1, "wallet": "0x1234567890123456789012345678901234567890"},
		},
		"artist_coins": {
			// Coin with 6 decimals
			{"ticker": "COIN1", "decimals": 6, "user_id": 1, "mint": "mint1"},
			// Coin with 8 decimals (like AUDIO)
			{"ticker": "COIN2", "decimals": 8, "user_id": 1, "mint": "mint2"},
			// Coin with 9 decimals
			{"ticker": "COIN3", "decimals": 9, "user_id": 1, "mint": "mint3"},
		},
		"artist_coin_stats": {
			{"mint": "mint1", "price": 1.0},
			{"mint": "mint2", "price": 1.0},
			{"mint": "mint3", "price": 1.0},
		},
		"sol_user_balances": {
			// Same raw balance, different decimals
			{"user_id": 1, "mint": "mint1", "balance": 1000000},    // 1.0 token (6 decimals)
			{"user_id": 1, "mint": "mint2", "balance": 100000000},  // 1.0 token (8 decimals)
			{"user_id": 1, "mint": "mint3", "balance": 1000000000}, // 1.0 token (9 decimals)
		},
	}

	database.Seed(pool, fixtures)

	// Run job
	job := NewBalanceHistoryJob(newTestConfig(), pool)
	err := job.run(ctx)
	require.NoError(t, err)

	// Verify all 3 have same USD value ($1.00 each)
	rows, err := pool.Query(ctx, "SELECT mint, balance_usd FROM user_balance_history WHERE user_id = 1 ORDER BY mint")
	require.NoError(t, err)
	defer rows.Close()

	var results []struct {
		mint       string
		balanceUsd float64
	}

	for rows.Next() {
		var r struct {
			mint       string
			balanceUsd float64
		}
		err := rows.Scan(&r.mint, &r.balanceUsd)
		require.NoError(t, err)
		results = append(results, r)
	}

	require.Equal(t, 3, len(results))

	// All should be $1.00 (1 token * $1 price)
	for _, r := range results {
		assert.InDelta(t, 1.0, r.balanceUsd, 0.01, "Expected $1.00 for mint %s", r.mint)
	}
}

func TestRecordBalanceHistoryOnlyTrackedCoins(t *testing.T) {
	pool := database.CreateTestDatabase(t, "test_api")
	defer pool.Close()

	ctx := context.Background()

	fixtures := database.FixtureMap{
		"users": {
			{"user_id": 1, "wallet": "0x1234567890123456789012345678901234567890"},
		},
		"artist_coins": {
			// Only COIN1 is in artist_coins
			{"ticker": "COIN1", "decimals": 6, "user_id": 1, "mint": "mint1"},
		},
		"artist_coin_stats": {
			{"mint": "mint1", "price": 1.0},
			{"mint": "mint2", "price": 2.0}, // Has price but not in artist_coins
		},
		"sol_user_balances": {
			{"user_id": 1, "mint": "mint1", "balance": 100000000}, // Should be recorded
			{"user_id": 1, "mint": "mint2", "balance": 100000000}, // Should NOT be recorded (not in artist_coins)
		},
	}

	database.Seed(pool, fixtures)

	// Run job
	job := NewBalanceHistoryJob(newTestConfig(), pool)
	err := job.run(ctx)
	require.NoError(t, err)

	// Verify only COIN1 was recorded (mint2 filtered out by JOIN artist_coins)
	var count int
	err = pool.QueryRow(ctx, "SELECT COUNT(*) FROM user_balance_history WHERE user_id = 1").Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 1, count, "Should only record tokens in artist_coins table")

	// Verify it's the correct mint
	var mint string
	err = pool.QueryRow(ctx, "SELECT mint FROM user_balance_history WHERE user_id = 1").Scan(&mint)
	require.NoError(t, err)
	assert.Equal(t, "mint1", mint)
}
