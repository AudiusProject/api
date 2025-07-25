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
				"user_id": 1,
				"wallet":  "0x1234567890123456789012345678901234567890",
			},
		},
		"associated_wallets": {
			{
				"id":      2,
				"user_id": 2,
				"chain":   "sol",
				"wallet":  "user_2_owner_1",
			},
			{
				"id":      3,
				"user_id": 3,
				"chain":   "sol",
				"wallet":  "user_3_owner_1",
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
		"sol_token_account_balance_changes": {
			// user_1: new $AUDIO member
			// $AUDIO claimable tokens account
			// received, sent it all, received more
			{
				"slot":            1,
				"signature":       "user_1_sig_1",
				"mint":            "9LzCMqDgTKYz9Drzqnpgee3SGa89up3a247ypMj2xrqM",
				"account":         "claimable",
				"owner":           "claimable_pda",
				"change":          1000000000,
				"balance":         1000000000,
				"block_timestamp": time.Now().Add(-time.Hour * 3),
			},
			{
				"slot":            2,
				"signature":       "user_1_sig_2",
				"mint":            "9LzCMqDgTKYz9Drzqnpgee3SGa89up3a247ypMj2xrqM",
				"account":         "claimable",
				"owner":           "claimable_pda",
				"change":          -1000000000,
				"balance":         0,
				"block_timestamp": time.Now().Add(-time.Hour * 2),
			},
			{
				"slot":            3,
				"signature":       "user_1_sig_3",
				"mint":            "9LzCMqDgTKYz9Drzqnpgee3SGa89up3a247ypMj2xrqM",
				"account":         "claimable",
				"owner":           "claimable_pda",
				"change":          100000000,
				"balance":         100000000,
				"block_timestamp": time.Now().Add(-time.Hour * 1),
			},
			// user_2: existing $AUDIO member
			// $AUDIO associated wallet 1
			{
				"slot":            1,
				"signature":       "user_2_sig_1",
				"mint":            "9LzCMqDgTKYz9Drzqnpgee3SGa89up3a247ypMj2xrqM",
				"account":         "user_2_account_1",
				"owner":           "user_2_owner_1",
				"change":          1000000000,
				"balance":         1000000000,
				"block_timestamp": time.Now().Add(-time.Hour * 25),
			},
			// $AUDIO associated wallet 2
			// sent it all today, but still a member because other account
			{
				"slot":            1,
				"signature":       "user_2_sig_2",
				"mint":            "9LzCMqDgTKYz9Drzqnpgee3SGa89up3a247ypMj2xrqM",
				"account":         "user_2_account_2",
				"owner":           "user_2_owner_1",
				"change":          1000000000,
				"balance":         1000000000,
				"block_timestamp": time.Now().Add(-time.Hour * 25),
			},
			{
				"slot":            2,
				"signature":       "user_2_sig_3",
				"mint":            "9LzCMqDgTKYz9Drzqnpgee3SGa89up3a247ypMj2xrqM",
				"account":         "user_2_account_2",
				"owner":           "user_2_owner_1",
				"change":          -1000000000,
				"balance":         0,
				"block_timestamp": time.Now().Add(-time.Hour * 2),
			},
			// user_3: existing $AUDIO member, former $USDC member
			// $AUDIO associated wallet 1
			// existing account
			{
				"slot":            1,
				"signature":       "user_3_sig_1",
				"mint":            "9LzCMqDgTKYz9Drzqnpgee3SGa89up3a247ypMj2xrqM",
				"account":         "user_3_account_1",
				"owner":           "user_3_owner_1",
				"change":          1000000000,
				"balance":         1000000000,
				"block_timestamp": time.Now().Add(-time.Hour * 25),
			},
			// $AUDIO associated wallet 2
			// new account, but already a member
			{
				"slot":            2,
				"signature":       "user_3_sig_2",
				"mint":            "9LzCMqDgTKYz9Drzqnpgee3SGa89up3a247ypMj2xrqM",
				"account":         "user_3_account_2",
				"owner":           "user_3_owner_1",
				"change":          1000000000,
				"balance":         1000000000,
				"block_timestamp": time.Now().Add(-time.Hour * 2),
			},
			// $USDC associated wallet
			// sent it all today, no longer a member
			{
				"slot":            1,
				"signature":       "user_3_sig_3",
				"mint":            "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v",
				"account":         "user_3_account_3",
				"owner":           "user_3_owner_1",
				"change":          1000000000,
				"balance":         1000000000,
				"block_timestamp": time.Now().Add(-time.Hour * 25),
			},
			{
				"slot":            2,
				"signature":       "user_3_sig_4",
				"mint":            "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v",
				"account":         "user_3_account_3",
				"owner":           "user_3_owner_1",
				"change":          -1000000000,
				"balance":         0,
				"block_timestamp": time.Now().Add(-time.Hour * 3),
			},
		},
	}

	database.Seed(app.pool, fixtures)

	app.birdeyeClient = &mockBirdeyeClient{}

	// User 1 is a new $AUDIO member, with users 2 and 3 being existing members.
	// User 3 is a former $USDC member, with no other members remaining
	// $AUDIO gained 1 member (2 remaining), $USDC lost 1 member (0 remaining)
	// $AUDIO gained 50% members, $USDC lost 100% members

	// no filters
	{
		status, body := testGet(t, app, "/v1/coins")
		assert.Equal(t, 200, status)

		jsonAssert(t, body, map[string]any{
			"data.0.ticker":                     "$AUDIO",
			"data.0.mint":                       "9LzCMqDgTKYz9Drzqnpgee3SGa89up3a247ypMj2xrqM",
			"data.0.decimals":                   8,
			"data.0.owner_id":                   trashid.MustEncodeHashID(1),
			"data.0.members":                    3,
			"data.0.members_24h_change_percent": 50.0,
			"data.1.ticker":                     "$USDC",
			"data.1.mint":                       "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v",
			"data.1.decimals":                   6,
			"data.1.owner_id":                   trashid.MustEncodeHashID(2),
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
			"data.0.mint":                       "9LzCMqDgTKYz9Drzqnpgee3SGa89up3a247ypMj2xrqM",
			"data.0.decimals":                   8,
			"data.0.owner_id":                   trashid.MustEncodeHashID(1),
			"data.0.members":                    3,
			"data.0.members_24h_change_percent": 50.0,
			"data.1":                            nil,
		})
	}

	// filter by mint
	{
		status, body := testGet(t, app, "/v1/coins?mint=EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v")
		assert.Equal(t, 200, status)

		jsonAssert(t, body, map[string]any{
			"data.0.ticker":                     "$USDC",
			"data.0.decimals":                   6,
			"data.0.mint":                       "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v",
			"data.0.owner_id":                   trashid.MustEncodeHashID(2),
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
			"data.0.mint":                       "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v",
			"data.0.decimals":                   6,
			"data.0.owner_id":                   trashid.MustEncodeHashID(2),
			"data.0.members":                    0,
			"data.0.members_24h_change_percent": -100.0,
			"data.1":                            nil,
		})
	}
}
