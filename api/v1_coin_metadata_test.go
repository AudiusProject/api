package api

import (
	"encoding/json"
	"testing"
	"time"

	"api.audius.co/database"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestV1CoinMetadata(t *testing.T) {
	app := emptyTestApp(t)

	fixtures := database.FixtureMap{
		"users": {
			{"user_id": 1, "handle": "bearartist", "is_current": true},
			{"user_id": 2, "handle": "bareartist", "is_current": true},
		},
		"artist_coins": {
			{
				"ticker":      "BEAR",
				"decimals":    9,
				"user_id":     1,
				"mint":        "9LzCMqDgTKYz9Drzqnpgee3SGa89up3a247ypMj2xrqM",
				"name":        "Bear Coin",
				"description": "A coin for bears",
				"logo_uri":    "https://creatornode.audius.co/content/bear-logo-cid",
				"created_at":  time.Now().Add(-time.Second),
			},
			{
				"ticker":     "BARE",
				"decimals":   9,
				"user_id":    2,
				"mint":       "3XyzCMqDgTKYz9Drzqnpgee3SGa89up3a247ypMj2xrqM",
				"name":       "Bare Coin",
				"created_at": time.Now().Add(-time.Second),
			},
		},
	}

	database.Seed(app.pool.Replicas[0], fixtures)

	// Serves the Metaplex standard document, unwrapped (no "data" envelope)
	{
		status, body := testGet(t, app, "/v1/coins/9LzCMqDgTKYz9Drzqnpgee3SGa89up3a247ypMj2xrqM/metadata")
		assert.Equal(t, 200, status)

		var metadata CoinMetadata
		require.NoError(t, json.Unmarshal(body, &metadata))
		assert.Equal(t, "Bear Coin", metadata.Name)
		assert.Equal(t, "BEAR", metadata.Symbol)
		assert.Equal(t, "A coin for bears", metadata.Description)
		assert.Equal(t, "https://creatornode.audius.co/content/bear-logo-cid", metadata.Image)
		assert.Equal(t, "http://localhost:1323/coins/BEAR", metadata.ExternalUrl)
		assert.NotNil(t, metadata.Attributes)

		// The on-chain uri points here, so wallets must not see our envelope
		var raw map[string]any
		require.NoError(t, json.Unmarshal(body, &raw))
		assert.NotContains(t, raw, "data")
	}

	// A coin with no stored description gets the launchpad sentence rebuilt,
	// matching LAUNCHPAD_COIN_DESCRIPTION in the web client. Coins launched
	// through the launchpad never persist one.
	{
		status, body := testGet(t, app, "/v1/coins/3XyzCMqDgTKYz9Drzqnpgee3SGa89up3a247ypMj2xrqM/metadata")
		assert.Equal(t, 200, status)

		var metadata CoinMetadata
		require.NoError(t, json.Unmarshal(body, &metadata))
		assert.Equal(t,
			"$BARE is an artist coin created by @bareartist on Audius. Learn more at http://localhost:1323/coins/BARE",
			metadata.Description)

		// Nullable columns serialize as empty strings, not null
		var raw map[string]any
		require.NoError(t, json.Unmarshal(body, &raw))
		assert.Equal(t, "", raw["image"])
		assert.Equal(t, []any{}, raw["attributes"])
	}

	// Unknown mint
	{
		status, _ := testGet(t, app, "/v1/coins/nonexistentmint/metadata")
		assert.Equal(t, 404, status)
	}
}
