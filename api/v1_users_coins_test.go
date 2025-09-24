package api

import (
	"testing"
	"time"

	"api.audius.co/database"
	"api.audius.co/trashid"
	"github.com/stretchr/testify/assert"
)

func TestUserCoins(t *testing.T) {
	app := emptyTestApp(t)

	fixtures := database.FixtureMap{
		"artist_coins": {
			{
				"ticker":     "$AUDIO",
				"decimals":   8,
				"user_id":    1,
				"mint":       "9LzCMqDgTKYz9Drzqnpgee3SGa89up3a247ypMj2xrqM",
				"created_at": time.Now().Add(-time.Second),
			},
			{
				"ticker":     "$USDC",
				"decimals":   6,
				"user_id":    2,
				"mint":       "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v",
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
		},
		"users": {
			{
				"user_id": 1,
				"wallet":  "0x1234567890123456789012345678901234567890",
			},
			{
				"user_id": 2,
				"wallet":  "0x098765432109876543210987654321",
			},
		},
		"associated_wallets": {
			// holds 10 AUDIO
			{
				"id":      1,
				"user_id": 1,
				"wallet":  "owner_wallet",
				"chain":   "sol",
			},
			// holds 10 USDC
			{
				"id":      2,
				"user_id": 1,
				"wallet":  "owner_wallet2",
				"chain":   "sol",
			},
			// holds 5 AUDIO
			{
				"id":      3,
				"user_id": 1,
				"wallet":  "owner_wallet3",
				"chain":   "sol",
			},
			// ignored (user 2)
			{
				"id":      4,
				"user_id": 2,
				"wallet":  "ignored_owner",
				"chain":   "sol",
			},
		},
		"sol_claimable_accounts": {
			// holds 3 AUDIO
			{
				"signature":        "claimable_signature_1",
				"account":          "claimable",
				"ethereum_address": "0x1234567890123456789012345678901234567890",
				"mint":             "9LzCMqDgTKYz9Drzqnpgee3SGa89up3a247ypMj2xrqM",
			},
		},
		"sol_token_account_balances": {
			{
				"mint":    "9LzCMqDgTKYz9Drzqnpgee3SGa89up3a247ypMj2xrqM",
				"account": "associated",
				"owner":   "owner_wallet",
				"balance": 1000000000, // 10 AUDIO
			},
			{
				"mint":    "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v",
				"account": "associated2",
				"owner":   "owner_wallet2",
				"balance": 7000000, // 7 USDC
			},
			{
				"mint":    "9LzCMqDgTKYz9Drzqnpgee3SGa89up3a247ypMj2xrqM",
				"account": "associated3",
				"owner":   "owner_wallet3",
				"balance": 500000000, // 5 AUDIO
			},
			{
				"mint":    "9LzCMqDgTKYz9Drzqnpgee3SGa89up3a247ypMj2xrqM",
				"account": "claimable",
				"owner":   "claimable_tokens_pda",
				"balance": 300000000, // 3 AUDIO
			},
			{
				"mint":    "9LzCMqDgTKYz9Drzqnpgee3SGa89up3a247ypMj2xrqM",
				"account": "ignored",
				"owner":   "ignored_owner",
				"balance": 500000000,
			},
		},
	}

	database.Seed(app.pool.Replicas[0], fixtures)

	status, body := testGet(t, app, "/v1/users/"+trashid.MustEncodeHashID(1)+"/coins")
	assert.Equal(t, 200, status)

	jsonAssert(t, body, map[string]any{
		"data.#":             2,
		"data.0.ticker":      "$AUDIO",
		"data.0.mint":        "9LzCMqDgTKYz9Drzqnpgee3SGa89up3a247ypMj2xrqM",
		"data.0.decimals":    8,
		"data.0.owner_id":    trashid.MustEncodeHashID(1),
		"data.0.balance":     1800000000, // 18 AUDIO
		"data.0.balance_usd": 180.0,      // Assuming $10 per AUDIO
		"data.1.ticker":      "$USDC",
		"data.1.mint":        "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v",
		"data.1.balance":     7000000, // 7 USDC
		"data.1.balance_usd": 7.0,
	})
}

func TestUserCoinsOrdering(t *testing.T) {
	app := emptyTestApp(t)

	fixtures := database.FixtureMap{
		"artist_coins": {
			{
				"ticker":     "$AUDIO",
				"decimals":   8,
				"user_id":    1, // User 1 owns AUDIO
				"mint":       "9LzCMqDgTKYz9Drzqnpgee3SGa89up3a247ypMj2xrqM",
				"created_at": time.Now().Add(-time.Second),
			},
			{
				"ticker":     "$MYCOIN",
				"decimals":   9,
				"user_id":    2, // User 2 owns MYCOIN
				"mint":       "mycoin_mint_address_123",
				"created_at": time.Now(),
			},
		},
		"artist_coin_stats": {
			{
				"mint":       "9LzCMqDgTKYz9Drzqnpgee3SGa89up3a247ypMj2xrqM",
				"price":      1.0, // $1 per AUDIO
				"updated_at": time.Now(),
			},
			{
				"mint":       "mycoin_mint_address_123",
				"price":      0.1, // $0.10 per MYCOIN
				"updated_at": time.Now(),
			},
		},
		"users": {
			{
				"user_id": 2,
				"wallet":  "0x098765432109876543210987654321",
			},
		},
		"associated_wallets": {
			// User 2 holds more AUDIO than their own MYCOIN
			{
				"id":      1,
				"user_id": 2,
				"wallet":  "owner_wallet",
				"chain":   "sol",
			},
			{
				"id":      2,
				"user_id": 2,
				"wallet":  "owner_wallet2",
				"chain":   "sol",
			},
		},
		"sol_token_account_balances": {
			// User 2 has 100 AUDIO (worth $100)
			{
				"mint":    "9LzCMqDgTKYz9Drzqnpgee3SGa89up3a247ypMj2xrqM",
				"account": "associated",
				"owner":   "owner_wallet",
				"balance": 100000000000, // 100 AUDIO
			},
			// User 2 has 50 MYCOIN (worth $5) - their own coin
			{
				"mint":    "mycoin_mint_address_123",
				"account": "associated2",
				"owner":   "owner_wallet2",
				"balance": 50000000000, // 50 MYCOIN
			},
		},
	}

	database.Seed(app.pool.Replicas[0], fixtures)

	status, body := testGet(t, app, "/v1/users/"+trashid.MustEncodeHashID(2)+"/coins")
	assert.Equal(t, 200, status)

	// Even though AUDIO has higher balance USD ($100 vs $5), MYCOIN should come first
	// because user 2 owns MYCOIN
	jsonAssert(t, body, map[string]any{
		"data.#":             2,
		"data.0.ticker":      "$MYCOIN", // Owned coin should come first
		"data.0.mint":        "mycoin_mint_address_123",
		"data.0.decimals":    9,
		"data.0.owner_id":    trashid.MustEncodeHashID(2),
		"data.0.balance":     50000000000, // 50 MYCOIN
		"data.0.balance_usd": 5.0,
		"data.1.ticker":      "$AUDIO", // Higher balance but not owned
		"data.1.mint":        "9LzCMqDgTKYz9Drzqnpgee3SGa89up3a247ypMj2xrqM",
		"data.1.balance":     100000000000, // 100 AUDIO
		"data.1.balance_usd": 100.0,
	})
}

func TestUserCoinsZeroBalanceOwned(t *testing.T) {
	app := emptyTestApp(t)

	fixtures := database.FixtureMap{
		"artist_coins": {
			{
				"ticker":     "$AUDIO",
				"decimals":   8,
				"user_id":    1, // User 1 owns AUDIO
				"mint":       "9LzCMqDgTKYz9Drzqnpgee3SGa89up3a247ypMj2xrqM",
				"created_at": time.Now().Add(-time.Second),
			},
			{
				"ticker":     "$ZEROCOIN",
				"decimals":   9,
				"user_id":    2, // User 2 owns ZEROCOIN but has zero balance
				"mint":       "zerocoin_mint_address_123",
				"created_at": time.Now(),
			},
		},
		"artist_coin_stats": {
			{
				"mint":       "9LzCMqDgTKYz9Drzqnpgee3SGa89up3a247ypMj2xrqM",
				"price":      1.0, // $1 per AUDIO
				"updated_at": time.Now(),
			},
			{
				"mint":       "zerocoin_mint_address_123",
				"price":      0.5, // $0.50 per ZEROCOIN
				"updated_at": time.Now(),
			},
		},
		"users": {
			{
				"user_id": 2,
				"wallet":  "0x098765432109876543210987654321",
			},
		},
		"associated_wallets": {
			// User 2 holds AUDIO but zero ZEROCOIN
			{
				"id":      1,
				"user_id": 2,
				"wallet":  "owner_wallet",
				"chain":   "sol",
			},
		},
		"sol_token_account_balances": {
			// User 2 has 50 AUDIO (worth $50)
			{
				"mint":    "9LzCMqDgTKYz9Drzqnpgee3SGa89up3a247ypMj2xrqM",
				"account": "associated",
				"owner":   "owner_wallet",
				"balance": 50000000000, // 50 AUDIO
			},
			// Note: No balance entry for ZEROCOIN - user has zero balance
		},
	}

	database.Seed(app.pool.Replicas[0], fixtures)

	status, body := testGet(t, app, "/v1/users/"+trashid.MustEncodeHashID(2)+"/coins")
	assert.Equal(t, 200, status)

	// ZEROCOIN should appear first even with zero balance because user 2 owns it
	jsonAssert(t, body, map[string]any{
		"data.#":             2,
		"data.0.ticker":      "$ZEROCOIN", // Owned coin should come first, even with zero balance
		"data.0.mint":        "zerocoin_mint_address_123",
		"data.0.decimals":    9,
		"data.0.owner_id":    trashid.MustEncodeHashID(2),
		"data.0.balance":     0, // Zero balance
		"data.0.balance_usd": 0.0,
		"data.1.ticker":      "$AUDIO", // Higher balance but not owned
		"data.1.mint":        "9LzCMqDgTKYz9Drzqnpgee3SGa89up3a247ypMj2xrqM",
		"data.1.balance":     50000000000, // 50 AUDIO
		"data.1.balance_usd": 50.0,
	})
}

func TestUserCoinsFilterZeroBalance(t *testing.T) {
	app := emptyTestApp(t)

	fixtures := database.FixtureMap{
		"artist_coins": {
			{
				"ticker":     "$AUDIO",
				"decimals":   8,
				"user_id":    1, // User 1 owns AUDIO
				"mint":       "9LzCMqDgTKYz9Drzqnpgee3SGa89up3a247ypMj2xrqM",
				"created_at": time.Now().Add(-time.Second),
			},
			{
				"ticker":     "$MYCOIN",
				"decimals":   9,
				"user_id":    2, // User 2 owns MYCOIN
				"mint":       "mycoin_mint_address_123",
				"created_at": time.Now(),
			},
			{
				"ticker":     "$OTHERCOIN",
				"decimals":   9,
				"user_id":    3, // User 3 owns OTHERCOIN (not user 2)
				"mint":       "othercoin_mint_address_123",
				"created_at": time.Now(),
			},
		},
		"artist_coin_stats": {
			{
				"mint":       "9LzCMqDgTKYz9Drzqnpgee3SGa89up3a247ypMj2xrqM",
				"price":      1.0, // $1 per AUDIO
				"updated_at": time.Now(),
			},
			{
				"mint":       "mycoin_mint_address_123",
				"price":      0.1, // $0.10 per MYCOIN
				"updated_at": time.Now(),
			},
			{
				"mint":       "othercoin_mint_address_123",
				"price":      0.5, // $0.50 per OTHERCOIN
				"updated_at": time.Now(),
			},
		},
		"users": {
			{
				"user_id": 2,
				"wallet":  "0x098765432109876543210987654321",
			},
		},
		"associated_wallets": {
			{
				"id":      1,
				"user_id": 2,
				"wallet":  "owner_wallet",
				"chain":   "sol",
			},
		},
		"sol_token_account_balances": {
			// User 2 has 50 AUDIO (worth $50)
			{
				"mint":    "9LzCMqDgTKYz9Drzqnpgee3SGa89up3a247ypMj2xrqM",
				"account": "associated",
				"owner":   "owner_wallet",
				"balance": 50000000000, // 50 AUDIO
			},
			// User 2 has zero balance in MYCOIN (their own coin) - should still appear
			// User 2 has zero balance in OTHERCOIN (not their coin) - should be filtered out
			// Note: No balance entries for MYCOIN or OTHERCOIN
		},
	}

	database.Seed(app.pool.Replicas[0], fixtures)

	status, body := testGet(t, app, "/v1/users/"+trashid.MustEncodeHashID(2)+"/coins")
	assert.Equal(t, 200, status)

	// Should only return 2 coins: MYCOIN (owned, zero balance) and AUDIO (positive balance)
	// OTHERCOIN should be filtered out because user doesn't own it and has zero balance
	jsonAssert(t, body, map[string]any{
		"data.#":             2, // Only 2 coins, not 3
		"data.0.ticker":      "$MYCOIN", // Owned coin should come first, even with zero balance
		"data.0.mint":        "mycoin_mint_address_123",
		"data.0.decimals":    9,
		"data.0.owner_id":    trashid.MustEncodeHashID(2),
		"data.0.balance":     0, // Zero balance
		"data.0.balance_usd": 0.0,
		"data.1.ticker":      "$AUDIO", // Positive balance
		"data.1.mint":        "9LzCMqDgTKYz9Drzqnpgee3SGa89up3a247ypMj2xrqM",
		"data.1.balance":     50000000000, // 50 AUDIO
		"data.1.balance_usd": 50.0,
	})
}
