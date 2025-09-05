package api

import (
	"testing"
	"time"

	"bridgerton.audius.co/database"
	"bridgerton.audius.co/trashid"
	"github.com/stretchr/testify/assert"
)

func TestV1Coin(t *testing.T) {
	app := emptyTestApp(t)

	fixtures := database.FixtureMap{
		"artist_coins": {
			{
				"ticker":     "$AUDIO",
				"decimals":   8,
				"user_id":    1,
				"mint":       "9LzCMqDgTKYz9Drzqnpgee3SGa89up3a247ypMj2xrqM",
				"name":       "Audius",
				"created_at": time.Now().Add(-time.Second),
			},
		},
	}

	database.Seed(app.pool.Replicas[0], fixtures)

	// Test /coins/:mint endpoint with mint address
	{
		status, body := testGet(t, app, "/v1/coins/9LzCMqDgTKYz9Drzqnpgee3SGa89up3a247ypMj2xrqM")
		assert.Equal(t, 200, status)

		jsonAssert(t, body, map[string]any{
			"data.ticker":   "$AUDIO",
			"data.mint":     "9LzCMqDgTKYz9Drzqnpgee3SGa89up3a247ypMj2xrqM",
			"data.decimals": 8,
			"data.name":     "Audius",
			"data.owner_id": trashid.MustEncodeHashID(1),
		})
	}

	// Test /coins/ticker/:ticker endpoint with ticker
	{
		status, body := testGet(t, app, "/v1/coins/ticker/$AUDIO")
		assert.Equal(t, 200, status)

		jsonAssert(t, body, map[string]any{
			"data.ticker":   "$AUDIO",
			"data.mint":     "9LzCMqDgTKYz9Drzqnpgee3SGa89up3a247ypMj2xrqM",
			"data.decimals": 8,
			"data.name":     "Audius",
			"data.owner_id": trashid.MustEncodeHashID(1),
		})
	}

	// Test with non-existent mint
	{
		status, body := testGet(t, app, "/v1/coins/nonexistent")
		assert.Equal(t, 404, status)
		assert.Contains(t, string(body), "no rows")
	}

	// Test with non-existent ticker
	{
		status, body := testGet(t, app, "/v1/coins/ticker/$NONEXISTENT")
		assert.Equal(t, 404, status)
		assert.Contains(t, string(body), "no rows")
	}
}
