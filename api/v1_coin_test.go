package api

import (
	"context"
	"fmt"
	"testing"
	"time"

	"bridgerton.audius.co/api/birdeye"
	"bridgerton.audius.co/database"
	"bridgerton.audius.co/trashid"
	"github.com/stretchr/testify/assert"
)

type mockBirdeyeClient struct{}

func (m *mockBirdeyeClient) GetTokenOverview(ctx context.Context, mint string, frames string) (*birdeye.TokenOverview, error) {
	switch mint {
	case "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v":
		return &birdeye.TokenOverview{
			Address: "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v",
			Price:   1.0,
		}, nil
	case "9LzCMqDgTKYz9Drzqnpgee3SGa89up3a247ypMj2xrqM":
		return &birdeye.TokenOverview{
			Address: "9LzCMqDgTKYz9Drzqnpgee3SGa89up3a247ypMj2xrqM",
			Price:   10.0,
		}, nil
	}
	return nil, fmt.Errorf("token not found")
}

func (m *mockBirdeyeClient) GetPrices(ctx context.Context, mints []string) (*birdeye.TokenPriceMap, error) {
	prices := make(birdeye.TokenPriceMap)
	for _, mint := range mints {
		switch mint {
		case "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v":
			prices[mint] = birdeye.TokenPriceData{
				Value: 1.0,
			}
		case "9LzCMqDgTKYz9Drzqnpgee3SGa89up3a247ypMj2xrqM":
			prices[mint] = birdeye.TokenPriceData{
				Value: 10.0,
			}
		default:
			return nil, fmt.Errorf("price not found for mint %s", mint)
		}
	}
	return &prices, nil
}

func TestGetCoin(t *testing.T) {
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
				"slot":            1,
				"signature":       "change2",
				"mint":            "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v",
				"account":         "associated2",
				"owner":           "owner_wallet2",
				"change":          1000000000,
				"balance":         1000000000,
				"block_timestamp": time.Now().Add(-time.Hour * 25),
			},
			{
				"slot":            2,
				"signature":       "change3",
				"mint":            "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v",
				"account":         "associated2",
				"owner":           "owner_wallet2",
				"change":          -1000000000,
				"balance":         0,
				"block_timestamp": time.Now().Add(-time.Hour * 1),
			},
			// wallet that was added today
			{
				"signature":       "change4",
				"mint":            "9LzCMqDgTKYz9Drzqnpgee3SGa89up3a247ypMj2xrqM",
				"account":         "associated",
				"owner":           "owner_wallet",
				"change":          1000000000,
				"balance":         1000000000,
				"block_timestamp": time.Now().Add(-time.Hour * 1),
			},
			// wallet added today that should be deduped as user 1 above
			{
				"signature":       "change5",
				"mint":            "9LzCMqDgTKYz9Drzqnpgee3SGa89up3a247ypMj2xrqM",
				"account":         "associated3",
				"owner":           "owner_wallet3",
				"change":          1000000000,
				"balance":         1000000000,
				"block_timestamp": time.Now().Add(-time.Hour * 1),
			},
		},
	}

	database.Seed(app.pool, fixtures)

	app.birdeyeClient = &mockBirdeyeClient{}

	// negative change
	{
		status, body := testGet(t, app, "/v1/coins/EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v")
		assert.Equal(t, 200, status)

		jsonAssert(t, body, map[string]any{
			"data.ticker":                     "$USDC",
			"data.owner_id":                   trashid.MustEncodeHashID(2),
			"data.mint":                       "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v",
			"data.members":                    0,
			"data.members_24h_change_percent": -100.0,
		})
	}

	// positive change
	{
		status, body := testGet(t, app, "/v1/coins/9LzCMqDgTKYz9Drzqnpgee3SGa89up3a247ypMj2xrqM")
		assert.Equal(t, 200, status)

		jsonAssert(t, body, map[string]any{
			"data.ticker":                     "$AUDIO",
			"data.owner_id":                   trashid.MustEncodeHashID(1),
			"data.mint":                       "9LzCMqDgTKYz9Drzqnpgee3SGa89up3a247ypMj2xrqM",
			"data.members":                    2,
			"data.members_24h_change_percent": 100.0,
		})
	}
}
