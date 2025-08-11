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
				"ticker":      "$USDC",
				"decimals":    6,
				"user_id":     2,
				"mint":        "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v",
				"logo_uri":    "https://example.com/usdc-logo.png",
				"description": "USDC is a stablecoin pegged to the US dollar.",
				"website":     "https://www.circle.com/en/usdc",
				"created_at":  time.Now(),
			},
		},
	}

	database.Seed(app.pool.Replicas[0], fixtures)

	app.birdeyeClient = &mockBirdeyeClient{}

	// no filters
	{
		status, body := testGet(t, app, "/v1/coins")
		assert.Equal(t, 200, status)

		jsonAssert(t, body, map[string]any{
			"data.0.ticker":      "$AUDIO",
			"data.0.mint":        "9LzCMqDgTKYz9Drzqnpgee3SGa89up3a247ypMj2xrqM",
			"data.0.decimals":    8,
			"data.0.owner_id":    trashid.MustEncodeHashID(1),
			"data.1.ticker":      "$USDC",
			"data.1.mint":        "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v",
			"data.1.decimals":    6,
			"data.1.owner_id":    trashid.MustEncodeHashID(2),
			"data.1.logo_uri":    "https://example.com/usdc-logo.png",
			"data.1.description": "USDC is a stablecoin pegged to the US dollar.",
			"data.1.website":     "https://www.circle.com/en/usdc",
		})
	}

	// filter by owner_id
	{
		status, body := testGet(t, app, "/v1/coins?owner_id="+trashid.MustEncodeHashID(1))
		assert.Equal(t, 200, status)

		jsonAssert(t, body, map[string]any{
			"data.0.ticker":   "$AUDIO",
			"data.0.mint":     "9LzCMqDgTKYz9Drzqnpgee3SGa89up3a247ypMj2xrqM",
			"data.0.decimals": 8,
			"data.0.owner_id": trashid.MustEncodeHashID(1),
			"data.1":          nil,
		})
	}

	// filter by mint
	{
		status, body := testGet(t, app, "/v1/coins?mint=EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v")
		assert.Equal(t, 200, status)

		jsonAssert(t, body, map[string]any{
			"data.0.ticker":   "$USDC",
			"data.0.decimals": 6,
			"data.0.mint":     "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v",
			"data.0.owner_id": trashid.MustEncodeHashID(2),
			"data.1":          nil,
		})
	}

	// limit and offset
	{
		status, body := testGet(t, app, "/v1/coins?limit=1&offset=1")
		assert.Equal(t, 200, status)

		jsonAssert(t, body, map[string]any{
			"data.0.ticker":   "$USDC",
			"data.0.mint":     "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v",
			"data.0.decimals": 6,
			"data.0.owner_id": trashid.MustEncodeHashID(2),
			"data.1":          nil,
		})
	}
}
