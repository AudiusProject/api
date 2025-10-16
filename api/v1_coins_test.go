package api

import (
	"testing"
	"time"

	"api.audius.co/database"
	"api.audius.co/trashid"
	"github.com/stretchr/testify/assert"
)

func TestGetCoins(t *testing.T) {
	app := emptyTestApp(t)

	fixtures := database.FixtureMap{
		"users": {
			{
				"user_id":   1,
				"handle":    "DJSpaceGoat",
				"handle_lc": "djspacegoat",
			},
			{
				"user_id":   2,
				"handle":    "BigArtistPerson",
				"handle_lc": "bigartistperson",
			},
		},
		"artist_coins": {
			{
				"ticker":     "AUDIO",
				"decimals":   8,
				"user_id":    1,
				"mint":       "9LzCMqDgTKYz9Drzqnpgee3SGa89up3a247ypMj2xrqM",
				"created_at": time.Now().Add(-time.Second),
			},
			{
				"ticker":      "USDC",
				"decimals":    6,
				"user_id":     2,
				"mint":        "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v",
				"logo_uri":    "https://example.com/usdc-logo.png",
				"description": "USDC is a stablecoin pegged to the US dollar.",
				"created_at":  time.Now(),
			},
		},
		"artist_coin_stats": {
			{
				"mint":             "9LzCMqDgTKYz9Drzqnpgee3SGa89up3a247ypMj2xrqM",
				"market_cap":       10000,
				"holder":           10,
				"total_volume_usd": 100,
				"price":            10.0,
				"created_at":       time.Now(),
			},
			{
				"mint":             "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v",
				"market_cap":       1000,
				"holder":           20,
				"total_volume_usd": 50,
				"price":            1.0,
				"created_at":       time.Now(),
			},
		},
	}

	database.Seed(app.pool.Replicas[0], fixtures)

	// filter by owner_id
	{
		status, body := testGet(t, app, "/v1/coins?owner_id="+trashid.MustEncodeHashID(1))
		assert.Equal(t, 200, status)

		jsonAssert(t, body, map[string]any{
			"data.0.ticker":   "AUDIO",
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
			"data.0.ticker":   "USDC",
			"data.0.decimals": 6,
			"data.0.mint":     "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v",
			"data.0.owner_id": trashid.MustEncodeHashID(2),
			"data.1":          nil,
		})
	}

	// query by handle
	{
		status, body := testGet(t, app, "/v1/coins?query=space")
		assert.Equal(t, 200, status)

		jsonAssert(t, body, map[string]any{
			"data.0.ticker": "AUDIO",
		})
	}

	// limit and offset
	{
		status, body := testGet(t, app, "/v1/coins?limit=1&offset=1")
		assert.Equal(t, 200, status)

		jsonAssert(t, body, map[string]any{
			"data.0.ticker":   "USDC",
			"data.0.mint":     "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v",
			"data.0.decimals": 6,
			"data.0.owner_id": trashid.MustEncodeHashID(2),
			"data.1":          nil,
		})
	}

	// default sort (market_cap desc)
	{
		status, body := testGet(t, app, "/v1/coins")
		assert.Equal(t, 200, status)

		jsonAssert(t, body, map[string]any{
			"data.0.ticker": "AUDIO",
		})
	}

	// market_cap asc
	{
		status, body := testGet(t, app, "/v1/coins?sort_method=market_cap&sort_direction=asc")
		assert.Equal(t, 200, status)

		jsonAssert(t, body, map[string]any{
			"data.0.ticker": "USDC",
		})
	}

	// price desc
	{
		status, body := testGet(t, app, "/v1/coins?sort_method=price&sort_direction=desc")
		assert.Equal(t, 200, status)

		jsonAssert(t, body, map[string]any{
			"data.0.ticker": "AUDIO",
		})
	}

	// price asc
	{
		status, body := testGet(t, app, "/v1/coins?sort_method=price&sort_direction=asc")
		assert.Equal(t, 200, status)

		jsonAssert(t, body, map[string]any{
			"data.0.ticker": "USDC",
		})
	}

	// volume desc
	{
		status, body := testGet(t, app, "/v1/coins?sort_method=volume&sort_direction=desc")
		assert.Equal(t, 200, status)

		jsonAssert(t, body, map[string]any{
			"data.0.ticker": "AUDIO",
		})
	}

	// volume asc
	{
		status, body := testGet(t, app, "/v1/coins?sort_method=volume&sort_direction=asc")
		assert.Equal(t, 200, status)

		jsonAssert(t, body, map[string]any{
			"data.0.ticker": "USDC",
		})
	}

	// holder desc
	{
		status, body := testGet(t, app, "/v1/coins?sort_method=holder&sort_direction=desc")
		assert.Equal(t, 200, status)

		jsonAssert(t, body, map[string]any{
			"data.0.ticker": "USDC",
		})
	}

	// holder asc
	{
		status, body := testGet(t, app, "/v1/coins?sort_method=holder&sort_direction=asc")
		assert.Equal(t, 200, status)

		jsonAssert(t, body, map[string]any{
			"data.0.ticker": "AUDIO",
		})
	}

	// created_at desc
	{
		status, body := testGet(t, app, "/v1/coins?sort_method=created_at&sort_direction=desc")
		assert.Equal(t, 200, status)

		jsonAssert(t, body, map[string]any{
			"data.0.ticker": "USDC",
		})
	}

	// created_at asc
	{
		status, body := testGet(t, app, "/v1/coins?sort_method=created_at&sort_direction=asc")
		assert.Equal(t, 200, status)

		jsonAssert(t, body, map[string]any{
			"data.0.ticker": "AUDIO",
		})
	}

}

func TestGetCoinsEmpty(t *testing.T) {
	app := emptyTestApp(t)

	{
		status, body := testGet(t, app, "/v1/coins")
		assert.Equal(t, 200, status)

		jsonAssert(t, body, map[string]any{
			"data.#": 0,
		})
	}
}
