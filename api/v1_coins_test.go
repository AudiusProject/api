package api

import (
	"testing"
	"time"

	"bridgerton.audius.co/database"
	"bridgerton.audius.co/trashid"
	"github.com/stretchr/testify/assert"
)

func TestGetCoins(t *testing.T) {
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
				"user_id": 3,
				"wallet":  "0x1234567890123456789012345678901234567890",
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
			// holds 0 USDC
			{
				"id":      2,
				"user_id": 2,
				"wallet":  "owner_wallet2",
				"chain":   "sol",
			},
			// holds 10 AUDIO, should be deduped as user 1
			{
				"id":      3,
				"user_id": 1,
				"wallet":  "owner_wallet3",
				"chain":   "sol",
			},
		},
		"sol_claimable_accounts": {
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
				"balance": 1000000000,
			},
			{
				"mint":    "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v",
				"account": "associated2",
				"owner":   "owner_wallet2",
				"balance": 0, // should be ignored as a member
			},
			{
				"mint":    "9LzCMqDgTKYz9Drzqnpgee3SGa89up3a247ypMj2xrqM",
				"account": "associated3",
				"owner":   "owner_wallet3", // should be deduped as user 1
				"balance": 1000000000,
			},
		},
		"sol_token_account_balance_changes": {
			// claimable tokens wallet that received tokens yesterday
			{
				"signature":       "change1",
				"mint":            "9LzCMqDgTKYz9Drzqnpgee3SGa89up3a247ypMj2xrqM",
				"account":         "claimable",
				"owner":           "claimable_pda",
				"change":          1000000000,
				"balance":         1000000000,
				"block_timestamp": time.Now().Add(-time.Hour * 25),
			},
			// wallet that was not empty yesterday, but empty today
			{
				"signature":       "change2",
				"mint":            "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v",
				"account":         "associated2",
				"owner":           "owner_wallet2",
				"change":          1000000000,
				"balance":         1000000000,
				"block_timestamp": time.Now().Add(-time.Hour * 25),
			},
		},
	}

	database.Seed(app.pool, fixtures)
	app.birdeyeClient = &MockBirdeyeClient{}

	// no filters
	{
		status, body := testGet(t, app, "/v1/coins")
		assert.Equal(t, 200, status)

		jsonAssert(t, body, map[string]any{
			"data.0.ticker":                     "$AUDIO",
			"data.0.owner_id":                   trashid.MustEncodeHashID(1),
			"data.0.mint":                       "9LzCMqDgTKYz9Drzqnpgee3SGa89up3a247ypMj2xrqM",
			"data.0.members":                    2,
			"data.0.members_24h_change_percent": 100.0,
			"data.1.ticker":                     "$USDC",
			"data.1.owner_id":                   trashid.MustEncodeHashID(2),
			"data.1.mint":                       "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v",
			"data.1.members":                    0,
			"data.1.members_24h_change_percent": -100.0,
		})
	}

	// filter by owner_id
	{
		status, body := testGet(t, app, "/v1/coins?owner_id="+trashid.MustEncodeHashID(1))
		assert.Equal(t, 200, status)

		jsonAssert(t, body, map[string]any{
			"data.0.ticker":                     "$AUDIO",
			"data.0.owner_id":                   trashid.MustEncodeHashID(1),
			"data.0.mint":                       "9LzCMqDgTKYz9Drzqnpgee3SGa89up3a247ypMj2xrqM",
			"data.0.members":                    2,
			"data.0.members_24h_change_percent": 100.0,
			"data.1":                            nil,
		})
	}

	// filter by mint
	{
		status, body := testGet(t, app, "/v1/coins?mint=EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v")
		assert.Equal(t, 200, status)

		jsonAssert(t, body, map[string]any{
			"data.0.ticker":                     "$USDC",
			"data.0.owner_id":                   trashid.MustEncodeHashID(2),
			"data.0.mint":                       "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v",
			"data.0.members":                    0,
			"data.0.members_24h_change_percent": -100.0,
			"data.1":                            nil,
		})
	}

	// limit and offset
	{
		status, body := testGet(t, app, "/v1/coins?limit=1&offset=1")
		assert.Equal(t, 200, status)

		jsonAssert(t, body, map[string]any{
			"data.0.ticker":                     "$USDC",
			"data.0.owner_id":                   trashid.MustEncodeHashID(2),
			"data.0.mint":                       "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v",
			"data.0.members":                    0,
			"data.0.members_24h_change_percent": -100.0,
			"data.1":                            nil,
		})
	}
}
