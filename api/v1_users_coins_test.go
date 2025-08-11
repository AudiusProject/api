package api

import (
	"testing"
	"time"

	"bridgerton.audius.co/database"
	"bridgerton.audius.co/trashid"
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
	app.birdeyeClient = &mockBirdeyeClient{}

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
