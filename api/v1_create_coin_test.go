package api

import (
	"context"
	"encoding/json"
	"testing"

	"bridgerton.audius.co/database"
	"bridgerton.audius.co/trashid"
	"github.com/stretchr/testify/assert"
)

func TestV1CreateCoin(t *testing.T) {
	app := emptyTestApp(t)
	database.Seed(app.pool.Replicas[0], database.FixtureMap{
		"users": {
			{
				"user_id": 1,
				"wallet":  "0x7d273271690538cf855e5b3002a0dd8c154bb060",
			},
		},
	})

	requestBody := CreateCoinBody{
		Mint:     "bearR26zyyB3fNQm5wWv1ZfN8MPQDUMwaAuoG79b1Yj",
		Ticker:   "$BEAR",
		Decimals: 9,
		Name:     "BEAR",
		LogoUri:  "https://example.com/bear-logo.png",
	}
	requestBodyBytes, err := json.Marshal(requestBody)
	assert.NoError(t, err)
	status, body := testPostWithWallet(t, app, "/v1/coins?user_id="+trashid.MustEncodeHashID(1), "0x7d273271690538cf855e5b3002a0dd8c154bb060", requestBodyBytes, map[string]string{
		"Content-Type": "application/json",
	})

	assert.Equal(t, 201, status)
	jsonAssert(t, body, map[string]any{
		"data.mint":     "bearR26zyyB3fNQm5wWv1ZfN8MPQDUMwaAuoG79b1Yj",
		"data.ticker":   "$BEAR",
		"data.user_id":  1,
		"data.decimals": 9,
		"data.name":     "BEAR",
		"data.logo_uri": "https://example.com/bear-logo.png",
	})

	// Verify the coin was actually created in the database
	var count int
	err = app.pool.QueryRow(context.Background(), "SELECT COUNT(*) FROM artist_coins WHERE mint = $1",
		"bearR26zyyB3fNQm5wWv1ZfN8MPQDUMwaAuoG79b1Yj").Scan(&count)
	assert.NoError(t, err)
	assert.Equal(t, 1, count)

	// Clean up
	app.pool.Exec(context.Background(), "DELETE FROM artist_coins WHERE mint = $1", "bearR26zyyB3fNQm5wWv1ZfN8MPQDUMwaAuoG79b1Yj")
}

func TestV1CreateCoin_DuplicateMint(t *testing.T) {
	app := emptyTestApp(t)
	database.Seed(app.pool.Replicas[0], database.FixtureMap{
		"users": {
			{
				"user_id": 1,
				"wallet":  "0x7d273271690538cf855e5b3002a0dd8c154bb060",
			},
		},
	})

	requestBody := CreateCoinBody{
		Mint:     "bearR26zyyB3fNQm5wWv1ZfN8MPQDUMwaAuoG79b1Yj",
		Ticker:   "$BEAR",
		Decimals: 9,
		Name:     "BEAR",
		LogoUri:  "https://example.com/bear-logo.png",
	}
	requestBodyBytes, err := json.Marshal(requestBody)
	assert.NoError(t, err)
	status, body := testPostWithWallet(t, app, "/v1/coins?user_id="+trashid.MustEncodeHashID(1), "0x7d273271690538cf855e5b3002a0dd8c154bb060", requestBodyBytes, map[string]string{
		"Content-Type": "application/json",
	})

	assert.Equal(t, 201, status)
	jsonAssert(t, body, map[string]any{
		"data.mint":     "bearR26zyyB3fNQm5wWv1ZfN8MPQDUMwaAuoG79b1Yj",
		"data.ticker":   "$BEAR",
		"data.user_id":  1,
		"data.decimals": 9,
		"data.name":     "BEAR",
		"data.logo_uri": "https://example.com/bear-logo.png",
	})

	// Verify the coin was actually created in the database
	var count int
	app.pool.QueryRow(context.Background(), "SELECT COUNT(*) FROM artist_coins WHERE mint = $1",
		"bearR26zyyB3fNQm5wWv1ZfN8MPQDUMwaAuoG79b1Yj").Scan(&count)
	assert.Equal(t, 1, count)

	// Try to create the coin again with a duplicate mint
	requestBody = CreateCoinBody{
		Mint:     "bearR26zyyB3fNQm5wWv1ZfN8MPQDUMwaAuoG79b1Yj",
		Ticker:   "$SNAKE",
		Decimals: 9,
		Name:     "SNAKE",
		LogoUri:  "https://example.com/snake-logo.png",
	}
	requestBodyBytes, _ = json.Marshal(requestBody)
	status, body = testPostWithWallet(t, app, "/v1/coins?user_id="+trashid.MustEncodeHashID(1), "0x7d273271690538cf855e5b3002a0dd8c154bb060", requestBodyBytes, map[string]string{
		"Content-Type": "application/json",
	})

	assert.Equal(t, 400, status)
	jsonAssert(t, body, map[string]any{
		"error": "Mint already exists",
	})

	// Try to create the coin again with a duplicate ticker
	requestBody = CreateCoinBody{
		Mint:     "snakeR26zyyB3fNQm5wWv1ZfN8MPQDUMwaAuoG79b1Y",
		Ticker:   "$BEAR",
		Decimals: 9,
		Name:     "BEAR",
		LogoUri:  "https://example.com/bear-logo.png",
	}
	requestBodyBytes, _ = json.Marshal(requestBody)
	status, body = testPostWithWallet(t, app, "/v1/coins?user_id="+trashid.MustEncodeHashID(1), "0x7d273271690538cf855e5b3002a0dd8c154bb060", requestBodyBytes, map[string]string{
		"Content-Type": "application/json",
	})

	assert.Equal(t, 400, status)
	jsonAssert(t, body, map[string]any{
		"error": "Ticker already exists",
	})

	// Clean up
	app.pool.Exec(context.Background(), "DELETE FROM artist_coins WHERE mint = $1", "bearR26zyyB3fNQm5wWv1ZfN8MPQDUMwaAuoG79b1Yj")
}
