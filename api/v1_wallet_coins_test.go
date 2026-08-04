package api

import (
	"testing"
	"time"

	"api.audius.co/database"
	"api.audius.co/trashid"
	"github.com/stretchr/testify/assert"
)

func TestWalletCoins(t *testing.T) {
	app := emptyTestApp(t)

	fixtures := database.FixtureMap{
		"artist_coins": {
			{
				"ticker":     "AUDIO",
				"decimals":   8,
				"user_id":    1,
				"mint":       "9LzCMqDgTKYz9Drzqnpgee3SGa89up3a247ypMj2xrqM",
				"created_at": time.Now().Add(-time.Second),
			},
			{
				"ticker":     "USDC",
				"decimals":   6,
				"user_id":    2,
				"mint":       "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v",
				"created_at": time.Now(),
			},
			{
				"ticker":     "MYCOIN",
				"decimals":   9,
				"user_id":    3,
				"mint":       "mycoin_mint_address_123",
				"created_at": time.Now(),
			},
		},
		"artist_coin_stats": {
			{
				"mint":       "9LzCMqDgTKYz9Drzqnpgee3SGa89up3a247ypMj2xrqM",
				"price":      10.0,
				"updated_at": time.Now(),
			},
			{
				"mint":       "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v",
				"price":      1.0,
				"updated_at": time.Now(),
			},
			{
				"mint":       "mycoin_mint_address_123",
				"price":      0.5,
				"updated_at": time.Now(),
			},
		},
		"sol_token_account_balances": {
			{
				"mint":    "9LzCMqDgTKYz9Drzqnpgee3SGa89up3a247ypMj2xrqM",
				"account": "account1",
				"owner":   "test_wallet_address",
				"balance": 1000000000, // 10 AUDIO
			},
			{
				"mint":    "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v",
				"account": "account2",
				"owner":   "test_wallet_address",
				"balance": 5000000, // 5 USDC
			},
			{
				"mint":    "mycoin_mint_address_123",
				"account": "account3",
				"owner":   "test_wallet_address",
				"balance": 20000000000, // 20 MYCOIN
			},
			// Different wallet - should not be included
			{
				"mint":    "9LzCMqDgTKYz9Drzqnpgee3SGa89up3a247ypMj2xrqM",
				"account": "account4",
				"owner":   "different_wallet",
				"balance": 5000000000,
			},
		},
	}

	database.Seed(app.writePool, fixtures)

	{
		status, body := testGet(t, app, "/v1/wallet/test_wallet_address/coins")
		assert.Equal(t, 200, status)

		jsonAssert(t, body, map[string]any{
			"data.#":             3,
			"data.0.ticker":      "AUDIO", // AUDIO comes first
			"data.0.mint":        "9LzCMqDgTKYz9Drzqnpgee3SGa89up3a247ypMj2xrqM",
			"data.0.decimals":    8,
			"data.0.owner_id":    trashid.MustEncodeHashID(1),
			"data.0.balance":     1000000000, // 10 AUDIO
			"data.0.balance_usd": 100.0,      // $10 per AUDIO
			"data.1.ticker":      "MYCOIN",   // Higher balance than USDC
			"data.1.mint":        "mycoin_mint_address_123",
			"data.1.decimals":    9,
			"data.1.owner_id":    trashid.MustEncodeHashID(3),
			"data.1.balance":     20000000000, // 20 MYCOIN
			"data.1.balance_usd": 10.0,        // $0.50 per MYCOIN
			"data.2.ticker":      "USDC",
			"data.2.mint":        "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v",
			"data.2.decimals":    6,
			"data.2.owner_id":    trashid.MustEncodeHashID(2),
			"data.2.balance":     5000000, // 5 USDC
			"data.2.balance_usd": 5.0,
		})
	}
}

func TestWalletCoinsNoBalance(t *testing.T) {
	app := emptyTestApp(t)

	fixtures := database.FixtureMap{
		"artist_coins": {
			{
				"ticker":     "AUDIO",
				"decimals":   8,
				"user_id":    1,
				"mint":       "9LzCMqDgTKYz9Drzqnpgee3SGa89up3a247ypMj2xrqM",
				"created_at": time.Now(),
			},
		},
		"sol_token_account_balances": {
			// Different wallet has balance, not our test wallet
			{
				"mint":    "9LzCMqDgTKYz9Drzqnpgee3SGa89up3a247ypMj2xrqM",
				"account": "account1",
				"owner":   "different_wallet",
				"balance": 1000000000,
			},
		},
	}

	database.Seed(app.writePool, fixtures)

	{
		status, body := testGet(t, app, "/v1/wallet/test_wallet_with_no_balance/coins")
		assert.Equal(t, 200, status)

		jsonAssert(t, body, map[string]any{
			"data.#": 0, // No coins with balance for this wallet
		})
	}
}

func TestWalletCoinsOrdering(t *testing.T) {
	app := emptyTestApp(t)

	fixtures := database.FixtureMap{
		"artist_coins": {
			{
				"ticker":     "AUDIO",
				"decimals":   8,
				"user_id":    1,
				"mint":       "9LzCMqDgTKYz9Drzqnpgee3SGa89up3a247ypMj2xrqM",
				"created_at": time.Now().Add(-time.Second),
			},
			{
				"ticker":     "HIGHVALUE",
				"decimals":   9,
				"user_id":    2,
				"mint":       "highvalue_mint_address_123",
				"created_at": time.Now(),
			},
			{
				"ticker":     "LOWVALUE",
				"decimals":   9,
				"user_id":    3,
				"mint":       "lowvalue_mint_address_123",
				"created_at": time.Now(),
			},
		},
		"artist_coin_stats": {
			{
				"mint":       "9LzCMqDgTKYz9Drzqnpgee3SGa89up3a247ypMj2xrqM",
				"price":      1.0,
				"updated_at": time.Now(),
			},
			{
				"mint":       "highvalue_mint_address_123",
				"price":      10.0,
				"updated_at": time.Now(),
			},
			{
				"mint":       "lowvalue_mint_address_123",
				"price":      0.1,
				"updated_at": time.Now(),
			},
		},
		"sol_token_account_balances": {
			// Small AUDIO balance (worth $5)
			{
				"mint":    "9LzCMqDgTKYz9Drzqnpgee3SGa89up3a247ypMj2xrqM",
				"account": "account1",
				"owner":   "test_wallet",
				"balance": 500000000, // 5 AUDIO
			},
			// Large HIGHVALUE balance (worth $1000)
			{
				"mint":    "highvalue_mint_address_123",
				"account": "account2",
				"owner":   "test_wallet",
				"balance": 100000000000, // 100 HIGHVALUE
			},
			// Medium LOWVALUE balance (worth $10)
			{
				"mint":    "lowvalue_mint_address_123",
				"account": "account3",
				"owner":   "test_wallet",
				"balance": 100000000000, // 100 LOWVALUE
			},
		},
	}

	database.Seed(app.writePool, fixtures)

	status, body := testGet(t, app, "/v1/wallet/test_wallet/coins")
	assert.Equal(t, 200, status)

	// AUDIO should come first regardless of balance, then by balance descending
	jsonAssert(t, body, map[string]any{
		"data.#":             3,
		"data.0.ticker":      "AUDIO", // AUDIO prioritized
		"data.0.mint":        "9LzCMqDgTKYz9Drzqnpgee3SGa89up3a247ypMj2xrqM",
		"data.0.balance":     500000000, // 5 AUDIO
		"data.0.balance_usd": 5.0,
		"data.1.ticker":      "HIGHVALUE", // Highest balance after AUDIO
		"data.1.mint":        "highvalue_mint_address_123",
		"data.1.balance":     100000000000, // 100 HIGHVALUE
		"data.1.balance_usd": 1000.0,
		"data.2.ticker":      "LOWVALUE", // Lower balance
		"data.2.mint":        "lowvalue_mint_address_123",
		"data.2.balance":     100000000000, // 100 LOWVALUE
		"data.2.balance_usd": 10.0,
	})
}

func TestWalletCoinsPagination(t *testing.T) {
	app := emptyTestApp(t)

	fixtures := database.FixtureMap{
		"artist_coins": {
			{
				"ticker":     "COIN1",
				"decimals":   9,
				"user_id":    1,
				"mint":       "mint1",
				"created_at": time.Now(),
			},
			{
				"ticker":     "COIN2",
				"decimals":   9,
				"user_id":    2,
				"mint":       "mint2",
				"created_at": time.Now(),
			},
			{
				"ticker":     "COIN3",
				"decimals":   9,
				"user_id":    3,
				"mint":       "mint3",
				"created_at": time.Now(),
			},
		},
		"artist_coin_stats": {
			{
				"mint":       "mint1",
				"price":      1.0,
				"updated_at": time.Now(),
			},
			{
				"mint":       "mint2",
				"price":      1.0,
				"updated_at": time.Now(),
			},
			{
				"mint":       "mint3",
				"price":      1.0,
				"updated_at": time.Now(),
			},
		},
		"sol_token_account_balances": {
			{
				"mint":    "mint1",
				"account": "account1",
				"owner":   "test_wallet",
				"balance": 3000000000, // Highest
			},
			{
				"mint":    "mint2",
				"account": "account2",
				"owner":   "test_wallet",
				"balance": 2000000000, // Middle
			},
			{
				"mint":    "mint3",
				"account": "account3",
				"owner":   "test_wallet",
				"balance": 1000000000, // Lowest
			},
		},
	}

	database.Seed(app.writePool, fixtures)

	// Test limit
	{
		status, body := testGet(t, app, "/v1/wallet/test_wallet/coins?limit=2")
		assert.Equal(t, 200, status)

		jsonAssert(t, body, map[string]any{
			"data.#":        2,
			"data.0.ticker": "COIN1", // Highest balance
			"data.1.ticker": "COIN2", // Second highest
		})
	}

	// Test offset
	{
		status, body := testGet(t, app, "/v1/wallet/test_wallet/coins?limit=2&offset=1")
		assert.Equal(t, 200, status)

		jsonAssert(t, body, map[string]any{
			"data.#":        2,
			"data.0.ticker": "COIN2", // Second highest (skipped first)
			"data.1.ticker": "COIN3", // Third highest
		})
	}

	// Test offset beyond results
	{
		status, body := testGet(t, app, "/v1/wallet/test_wallet/coins?offset=10")
		assert.Equal(t, 200, status)

		jsonAssert(t, body, map[string]any{
			"data.#": 0, // No results
		})
	}
}

func TestWalletCoinsWithPoolPricing(t *testing.T) {
	app := emptyTestApp(t)

	fixtures := database.FixtureMap{
		"artist_coins": {
			{
				"ticker":     "POOLCOIN",
				"decimals":   9,
				"user_id":    1,
				"mint":       "poolcoin_mint",
				"created_at": time.Now(),
			},
		},
		"artist_coin_pools": {
			// Using pool price instead of stats price
			{
				"address":   "pool_address_123",
				"base_mint": "poolcoin_mint",
				"price_usd": 2.5,
			},
		},
		"sol_token_account_balances": {
			{
				"mint":    "poolcoin_mint",
				"account": "account1",
				"owner":   "test_wallet",
				"balance": 10000000000, // 10 POOLCOIN
			},
		},
	}

	database.Seed(app.writePool, fixtures)

	{
		status, body := testGet(t, app, "/v1/wallet/test_wallet/coins")
		assert.Equal(t, 200, status)

		jsonAssert(t, body, map[string]any{
			"data.#":             1,
			"data.0.ticker":      "POOLCOIN",
			"data.0.mint":        "poolcoin_mint",
			"data.0.balance":     10000000000, // 10 POOLCOIN
			"data.0.balance_usd": 25.0,        // 10 * $2.50
		})
	}
}

// TestWalletCoinsAudioPricingFromStats verifies that AUDIO gets its price from artist_coin_stats
// when there are no pools (DAMM V2, DBC, or artist_coin_pools). This is the expected behavior
// for AUDIO since it's the quote token and doesn't have its own pools.
func TestWalletCoinsAudioPricingFromStats(t *testing.T) {
	app := emptyTestApp(t)

	audioMint := "9LzCMqDgTKYz9Drzqnpgee3SGa89up3a247ypMj2xrqM" // AUDIO mint

	fixtures := database.FixtureMap{
		"artist_coins": {
			{
				"ticker":     "AUDIO",
				"decimals":   8,
				"user_id":    1,
				"mint":       audioMint,
				"created_at": time.Now(),
			},
		},
		"artist_coin_stats": {
			// AUDIO price anchor (updated by AudioPriceJob from the AUDIO/USDC pool)
			{
				"mint":       audioMint,
				"price":      0.25, // $0.25 per AUDIO
				"updated_at": time.Now(),
			},
		},
		// Explicitly NO pools - AUDIO should fall back to artist_coin_stats
		"sol_token_account_balances": {
			{
				"mint":    audioMint,
				"account": "account1",
				"owner":   "test_wallet",
				"balance": 77479000000, // 774.79 AUDIO (matching the UI screenshot)
			},
		},
	}

	database.Seed(app.writePool, fixtures)

	{
		status, body := testGet(t, app, "/v1/wallet/test_wallet/coins")
		assert.Equal(t, 200, status)

		jsonAssert(t, body, map[string]any{
			"data.#":             1,
			"data.0.ticker":      "AUDIO",
			"data.0.mint":        audioMint,
			"data.0.balance":     77479000000, // 774.79 AUDIO
			"data.0.balance_usd": 193.6975,    // 774.79 * $0.25 = $193.6975
		})
	}
}

func TestWalletCoinsMultipleAccounts(t *testing.T) {
	app := emptyTestApp(t)

	fixtures := database.FixtureMap{
		"artist_coins": {
			{
				"ticker":     "AUDIO",
				"decimals":   8,
				"user_id":    1,
				"mint":       "9LzCMqDgTKYz9Drzqnpgee3SGa89up3a247ypMj2xrqM",
				"created_at": time.Now(),
			},
		},
		"artist_coin_stats": {
			{
				"mint":       "9LzCMqDgTKYz9Drzqnpgee3SGa89up3a247ypMj2xrqM",
				"price":      1.0,
				"updated_at": time.Now(),
			},
		},
		"sol_token_account_balances": {
			// Same wallet, multiple accounts with same mint
			{
				"mint":    "9LzCMqDgTKYz9Drzqnpgee3SGa89up3a247ypMj2xrqM",
				"account": "account1",
				"owner":   "test_wallet",
				"balance": 5000000000, // 50 AUDIO
			},
			{
				"mint":    "9LzCMqDgTKYz9Drzqnpgee3SGa89up3a247ypMj2xrqM",
				"account": "account2",
				"owner":   "test_wallet",
				"balance": 3000000000, // 30 AUDIO
			},
			{
				"mint":    "9LzCMqDgTKYz9Drzqnpgee3SGa89up3a247ypMj2xrqM",
				"account": "account3",
				"owner":   "test_wallet",
				"balance": 2000000000, // 20 AUDIO
			},
		},
	}

	database.Seed(app.writePool, fixtures)

	{
		status, body := testGet(t, app, "/v1/wallet/test_wallet/coins")
		assert.Equal(t, 200, status)

		// Balance should be sum of all accounts
		jsonAssert(t, body, map[string]any{
			"data.#":             1,
			"data.0.ticker":      "AUDIO",
			"data.0.balance":     10000000000, // 50 + 30 + 20 = 100 AUDIO
			"data.0.balance_usd": 100.0,       // 100 * $1
		})
	}
}
