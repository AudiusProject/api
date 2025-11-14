package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"testing"
	"time"

	"api.audius.co/database"
	"api.audius.co/trashid"
	"github.com/stretchr/testify/assert"
)

func TestV1CreateCoin(t *testing.T) {
	app := emptyTestApp(t)
	database.Seed(app.pool.Replicas[0], database.FixtureMap{
		"users": {
			{
				"user_id":     1,
				"wallet":      "0x7d273271690538cf855e5b3002a0dd8c154bb060",
				"is_verified": true,
			},
		},
	})

	requestBody := CreateCoinBody{
		Mint:        "bearR26zyyB3fNQm5wWv1ZfN8MPQDUMwaAuoG79b1Yj",
		Ticker:      "BEAR",
		Decimals:    9,
		Name:        "BEAR",
		LogoUri:     "https://example.com/bear-logo.png",
		Description: "A majestic bear token for wildlife conservation",
	}
	requestBodyBytes, err := json.Marshal(requestBody)
	assert.NoError(t, err)
	status, body := testPostWithWallet(t, app, "/v1/coins?user_id="+trashid.MustEncodeHashID(1), "0x7d273271690538cf855e5b3002a0dd8c154bb060", requestBodyBytes, map[string]string{
		"Content-Type": "application/json",
	})

	assert.Equal(t, 201, status)
	jsonAssert(t, body, map[string]any{
		"data.mint":        "bearR26zyyB3fNQm5wWv1ZfN8MPQDUMwaAuoG79b1Yj",
		"data.ticker":      "BEAR",
		"data.user_id":     1,
		"data.decimals":    9,
		"data.name":        "BEAR",
		"data.logo_uri":    "https://example.com/bear-logo.png",
		"data.description": "A majestic bear token for wildlife conservation",
	})

	// Verify the coin was actually created by fetching it via API
	status, body = testGet(t, app, "/v1/coins/bearR26zyyB3fNQm5wWv1ZfN8MPQDUMwaAuoG79b1Yj")
	assert.Equal(t, 200, status)
	jsonAssert(t, body, map[string]any{
		"data.mint":   "bearR26zyyB3fNQm5wWv1ZfN8MPQDUMwaAuoG79b1Yj",
		"data.ticker": "BEAR",
		"data.name":   "BEAR",
	})

}

func TestV1CreateCoin_StoresLinks(t *testing.T) {
	app := emptyTestApp(t)
	database.Seed(app.pool.Replicas[0], database.FixtureMap{
		"users": {
			{
				"user_id":     2,
				"wallet":      "0xc3d1d41e6872ffbd15c473d14fc3a9250be5b5e0",
				"is_verified": true,
			},
		},
	})

	link1Val := "https://example.com/alpha"
	link2Val := "https://example.com/beta"
	link3Val := ""

	requestBody := CreateCoinBody{
		Mint:        "lionR26zyyB3fNQm5wWv1ZfN8MPQDUMwaAuoG79b1Yj",
		Ticker:      "LION",
		Decimals:    9,
		Name:        "LION",
		LogoUri:     "https://example.com/lion-logo.png",
		Description: "Roaring with automatic social links",
		Link1:       &link1Val,
		Link2:       &link2Val,
		Link3:       &link3Val,
	}
	requestBodyBytes, err := json.Marshal(requestBody)
	assert.NoError(t, err)

	status, _ := testPostWithWallet(t, app, "/v1/coins?user_id="+trashid.MustEncodeHashID(2), "0xc3d1d41e6872ffbd15c473d14fc3a9250be5b5e0", requestBodyBytes, map[string]string{
		"Content-Type": "application/json",
	})

	assert.Equal(t, 201, status)

	var dbLink1, dbLink2, dbLink3, dbLink4 sql.NullString
	err = app.pool.QueryRow(context.Background(), `
		SELECT link_1, link_2, link_3, link_4
		FROM artist_coins
		WHERE mint = $1
	`, requestBody.Mint).Scan(&dbLink1, &dbLink2, &dbLink3, &dbLink4)
	assert.NoError(t, err)

	assert.True(t, dbLink1.Valid)
	assert.Equal(t, "https://example.com/alpha", dbLink1.String)

	assert.True(t, dbLink2.Valid)
	assert.Equal(t, "https://example.com/beta", dbLink2.String)

	assert.False(t, dbLink3.Valid)
	assert.False(t, dbLink4.Valid)
}

func TestV1CreateCoin_DuplicateMint(t *testing.T) {
	app := emptyTestApp(t)
	database.Seed(app.pool.Replicas[0], database.FixtureMap{
		"users": {
			{
				"user_id":     1,
				"wallet":      "0x7d273271690538cf855e5b3002a0dd8c154bb060",
				"is_verified": true,
			},
			{
				"user_id":     2,
				"wallet":      "0xc3d1d41e6872ffbd15c473d14fc3a9250be5b5e0",
				"is_verified": true,
			},
		},
	})

	requestBody := CreateCoinBody{
		Mint:        "bearR26zyyB3fNQm5wWv1ZfN8MPQDUMwaAuoG79b1Yj",
		Ticker:      "BEAR",
		Decimals:    9,
		Name:        "BEAR",
		LogoUri:     "https://example.com/bear-logo.png",
		Description: "A majestic bear token for wildlife conservation",
	}
	requestBodyBytes, err := json.Marshal(requestBody)
	assert.NoError(t, err)
	status, body := testPostWithWallet(t, app, "/v1/coins?user_id="+trashid.MustEncodeHashID(1), "0x7d273271690538cf855e5b3002a0dd8c154bb060", requestBodyBytes, map[string]string{
		"Content-Type": "application/json",
	})

	assert.Equal(t, 201, status)
	jsonAssert(t, body, map[string]any{
		"data.mint":        "bearR26zyyB3fNQm5wWv1ZfN8MPQDUMwaAuoG79b1Yj",
		"data.ticker":      "BEAR",
		"data.user_id":     1,
		"data.decimals":    9,
		"data.name":        "BEAR",
		"data.logo_uri":    "https://example.com/bear-logo.png",
		"data.description": "A majestic bear token for wildlife conservation",
	})

	// Verify the coin was actually created by fetching it via API
	status, body = testGet(t, app, "/v1/coins/bearR26zyyB3fNQm5wWv1ZfN8MPQDUMwaAuoG79b1Yj")
	assert.Equal(t, 200, status)
	jsonAssert(t, body, map[string]any{
		"data.mint":   "bearR26zyyB3fNQm5wWv1ZfN8MPQDUMwaAuoG79b1Yj",
		"data.ticker": "BEAR",
		"data.name":   "BEAR",
	})

	// Try to create the coin again with a duplicate mint using a different user
	requestBody = CreateCoinBody{
		Mint:        "bearR26zyyB3fNQm5wWv1ZfN8MPQDUMwaAuoG79b1Yj",
		Ticker:      "SNAKE",
		Decimals:    9,
		Name:        "SNAKE",
		LogoUri:     "https://example.com/snake-logo.png",
		Description: "A cunning snake token for reptilian enthusiasts",
	}
	requestBodyBytes, _ = json.Marshal(requestBody)
	status, body = testPostWithWallet(t, app, "/v1/coins?user_id="+trashid.MustEncodeHashID(2), "0xc3d1d41e6872ffbd15c473d14fc3a9250be5b5e0", requestBodyBytes, map[string]string{
		"Content-Type": "application/json",
	})

	assert.Equal(t, 400, status)
	jsonAssert(t, body, map[string]any{
		"error": "Mint already exists",
	})

	// Try to create the coin again with a duplicate ticker using a different user
	requestBody = CreateCoinBody{
		Mint:        "snakeR26zyyB3fNQm5wWv1ZfN8MPQDUMwaAuoG79b1Y",
		Ticker:      "BEAR",
		Decimals:    9,
		Name:        "BEAR",
		LogoUri:     "https://example.com/bear-logo.png",
		Description: "A majestic bear token for wildlife conservation",
	}
	requestBodyBytes, _ = json.Marshal(requestBody)
	status, body = testPostWithWallet(t, app, "/v1/coins?user_id="+trashid.MustEncodeHashID(2), "0xc3d1d41e6872ffbd15c473d14fc3a9250be5b5e0", requestBodyBytes, map[string]string{
		"Content-Type": "application/json",
	})

	assert.Equal(t, 400, status)
	jsonAssert(t, body, map[string]any{
		"error": "Ticker already exists",
	})

}

func TestV1CreateCoin_UnverifiedUser(t *testing.T) {
	app := emptyTestApp(t)
	database.Seed(app.pool.Replicas[0], database.FixtureMap{
		"users": {
			{
				"user_id":     2,
				"wallet":      "0xc3d1d41e6872ffbd15c473d14fc3a9250be5b5e0", // Use existing wallet with signature data
				"is_verified": false,                                        // User is not verified
			},
		},
	})

	requestBody := CreateCoinBody{
		Mint:        "bearR26zyyB3fNQm5wWv1ZfN8MPQDUMwaAuoG79b1Yj",
		Ticker:      "BEAR",
		Decimals:    9,
		Name:        "BEAR",
		LogoUri:     "https://example.com/bear-logo.png",
		Description: "A majestic bear token for wildlife conservation",
	}
	requestBodyBytes, err := json.Marshal(requestBody)
	assert.NoError(t, err)

	status, body := testPostWithWallet(t, app, "/v1/coins?user_id="+trashid.MustEncodeHashID(2), "0xc3d1d41e6872ffbd15c473d14fc3a9250be5b5e0", requestBodyBytes, map[string]string{
		"Content-Type": "application/json",
	})

	assert.Equal(t, 400, status)
	jsonAssert(t, body, map[string]any{
		"error": "User must be verified to create coins",
	})

	// Verify the coin was NOT created by trying to fetch it via API
	status, _ = testGet(t, app, "/v1/coins/bearR26zyyB3fNQm5wWv1ZfN8MPQDUMwaAuoG79b1Yj")
	assert.Equal(t, 404, status)
}

func TestV1CreateCoin_DeactivatedUser(t *testing.T) {
	app := emptyTestApp(t)
	database.Seed(app.pool.Replicas[0], database.FixtureMap{
		"users": {
			{
				"user_id":        3,
				"wallet":         "0x4954d18926ba0ed9378938444731be4e622537b2", // Use existing wallet with signature data
				"is_verified":    true,
				"is_deactivated": true, // User is deactivated
			},
		},
	})

	requestBody := CreateCoinBody{
		Mint:        "bearR26zyyB3fNQm5wWv1ZfN8MPQDUMwaAuoG79b1Yj",
		Ticker:      "BEAR",
		Decimals:    9,
		Name:        "BEAR",
		LogoUri:     "https://example.com/bear-logo.png",
		Description: "A majestic bear token for wildlife conservation",
	}
	requestBodyBytes, err := json.Marshal(requestBody)
	assert.NoError(t, err)

	status, body := testPostWithWallet(t, app, "/v1/coins?user_id="+trashid.MustEncodeHashID(3), "0x4954d18926ba0ed9378938444731be4e622537b2", requestBodyBytes, map[string]string{
		"Content-Type": "application/json",
	})

	assert.Equal(t, 404, status)
	jsonAssert(t, body, map[string]any{
		"error": "no rows in result set",
	})

	// Verify the coin was NOT created by trying to fetch it via API
	status, _ = testGet(t, app, "/v1/coins/bearR26zyyB3fNQm5wWv1ZfN8MPQDUMwaAuoG79b1Yj")
	assert.Equal(t, 404, status)
}

func TestV1CreateCoin_UserAlreadyHasCoin(t *testing.T) {
	app := emptyTestApp(t)
	database.Seed(app.pool.Replicas[0], database.FixtureMap{
		"users": {
			{
				"user_id":     4,
				"wallet":      "0x7d273271690538cf855e5b3002a0dd8c154bb060",
				"is_verified": true,
			},
		},
		"artist_coins": {
			{
				"mint":       "existingMint12345678901234567890123456789012",
				"ticker":     "FIRST",
				"user_id":    4,
				"decimals":   9,
				"name":       "First Coin",
				"logo_uri":   "https://example.com/first-logo.png",
				"created_at": time.Now(),
			},
		},
	})

	// Try to create a second coin for the same user
	requestBody := CreateCoinBody{
		Mint:        "secondMint123456789012345678901234567890123",
		Ticker:      "SECOND",
		Decimals:    9,
		Name:        "Second Coin",
		LogoUri:     "https://example.com/second-logo.png",
		Description: "A second token for testing purposes",
	}
	requestBodyBytes, err := json.Marshal(requestBody)
	assert.NoError(t, err)

	status, _ := testPostWithWallet(t, app, "/v1/coins?user_id="+trashid.MustEncodeHashID(4), "0x7d273271690538cf855e5b3002a0dd8c154bb060", requestBodyBytes, map[string]string{
		"Content-Type": "application/json",
	})

	assert.Equal(t, 400, status)

	// Verify the second coin was NOT created
	status, _ = testGet(t, app, "/v1/coins/secondMint123456789012345678901234567890123")
	assert.Equal(t, 404, status)
}

func TestV1CreateCoin_NameTooLong(t *testing.T) {
	app := emptyTestApp(t)
	database.Seed(app.pool.Replicas[0], database.FixtureMap{
		"users": {
			{
				"user_id":     1,
				"wallet":      "0x7d273271690538cf855e5b3002a0dd8c154bb060",
				"is_verified": true,
			},
		},
	})

	// Create a name that is longer than 32 characters
	longName := "ThisIsAVeryLongCoinNameThatExceedsTheThirtyTwoCharacterLimit"
	assert.Greater(t, len(longName), 32)

	requestBody := CreateCoinBody{
		Mint:        "bearR26zyyB3fNQm5wWv1ZfN8MPQDUMwaAuoG79b1Yj",
		Ticker:      "BEAR",
		Decimals:    9,
		Name:        longName,
		LogoUri:     "https://example.com/bear-logo.png",
		Description: "A majestic bear token",
	}
	requestBodyBytes, err := json.Marshal(requestBody)
	assert.NoError(t, err)

	status, body := testPostWithWallet(t, app, "/v1/coins?user_id="+trashid.MustEncodeHashID(1), "0x7d273271690538cf855e5b3002a0dd8c154bb060", requestBodyBytes, map[string]string{
		"Content-Type": "application/json",
	})

	// This should now return 400 Bad Request due to name length validation
	assert.Equal(t, 400, status)
	jsonAssert(t, body, map[string]any{
		"error": "Name is invalid",
	})
}

func TestV1CreateCoin_DescriptionTooLong(t *testing.T) {
	app := emptyTestApp(t)
	database.Seed(app.pool.Replicas[0], database.FixtureMap{
		"users": {
			{
				"user_id":     1,
				"wallet":      "0x7d273271690538cf855e5b3002a0dd8c154bb060",
				"is_verified": true,
			},
		},
	})

	// Create a description that is longer than 2500 characters
	longDescription := ""
	for len(longDescription) <= 2500 {
		longDescription += "This is a very long description that will exceed the 2500 character limit when repeated many times. "
	}
	assert.Greater(t, len(longDescription), 2500)

	requestBody := CreateCoinBody{
		Mint:        "bearR26zyyB3fNQm5wWv1ZfN8MPQDUMwaAuoG79b1Yj",
		Ticker:      "BEAR",
		Decimals:    9,
		Name:        "BEAR",
		LogoUri:     "https://example.com/bear-logo.png",
		Description: longDescription,
	}
	requestBodyBytes, err := json.Marshal(requestBody)
	assert.NoError(t, err)

	status, body := testPostWithWallet(t, app, "/v1/coins?user_id="+trashid.MustEncodeHashID(1), "0x7d273271690538cf855e5b3002a0dd8c154bb060", requestBodyBytes, map[string]string{
		"Content-Type": "application/json",
	})

	// This should now return 400 Bad Request due to description length validation
	assert.Equal(t, 400, status)
	jsonAssert(t, body, map[string]any{
		"error": "Description is invalid",
	})
}

func TestV1CreateCoin_MissingRequiredFields(t *testing.T) {
	app := emptyTestApp(t)
	database.Seed(app.pool.Replicas[0], database.FixtureMap{
		"users": {
			{
				"user_id":     1,
				"wallet":      "0x7d273271690538cf855e5b3002a0dd8c154bb060",
				"is_verified": true,
			},
		},
	})

	requestBody := CreateCoinBody{
		// Missing required fields: Mint, Ticker, Name, Decimals
		LogoUri:     "https://example.com/logo.png",
		Description: "A test coin",
	}
	requestBodyBytes, err := json.Marshal(requestBody)
	assert.NoError(t, err)

	status, _ := testPostWithWallet(t, app, "/v1/coins?user_id="+trashid.MustEncodeHashID(1), "0x7d273271690538cf855e5b3002a0dd8c154bb060", requestBodyBytes, map[string]string{
		"Content-Type": "application/json",
	})

	// This should return 400 Bad Request due to missing required fields
	assert.Equal(t, 400, status)
	// The validator will return an error for the first missing required field it encounters
}

func TestV1CreateCoin_InvalidMintLength(t *testing.T) {
	app := emptyTestApp(t)
	database.Seed(app.pool.Replicas[0], database.FixtureMap{
		"users": {
			{
				"user_id":     1,
				"wallet":      "0x7d273271690538cf855e5b3002a0dd8c154bb060",
				"is_verified": true,
			},
		},
	})

	requestBody := CreateCoinBody{
		Mint:        "shortMint", // Too short (should be 44 chars)
		Ticker:      "BEAR",
		Decimals:    9,
		Name:        "BEAR",
		LogoUri:     "https://example.com/bear-logo.png",
		Description: "A majestic bear token",
	}
	requestBodyBytes, err := json.Marshal(requestBody)
	assert.NoError(t, err)

	status, body := testPostWithWallet(t, app, "/v1/coins?user_id="+trashid.MustEncodeHashID(1), "0x7d273271690538cf855e5b3002a0dd8c154bb060", requestBodyBytes, map[string]string{
		"Content-Type": "application/json",
	})

	assert.Equal(t, 400, status)
	jsonAssert(t, body, map[string]any{
		"error": "Mint is invalid",
	})
}

func TestV1CreateCoin_InvalidTickerCharacters(t *testing.T) {
	app := emptyTestApp(t)
	database.Seed(app.pool.Replicas[0], database.FixtureMap{
		"users": {
			{
				"user_id":     1,
				"wallet":      "0x7d273271690538cf855e5b3002a0dd8c154bb060",
				"is_verified": true,
			},
		},
	})

	requestBody := CreateCoinBody{
		Mint:        "bearR26zyyB3fNQm5wWv1ZfN8MPQDUMwaAuoG79b1Yj",
		Ticker:      "BE@R", // Invalid character @
		Decimals:    9,
		Name:        "BEAR",
		LogoUri:     "https://example.com/bear-logo.png",
		Description: "A majestic bear token",
	}
	requestBodyBytes, err := json.Marshal(requestBody)
	assert.NoError(t, err)

	status, body := testPostWithWallet(t, app, "/v1/coins?user_id="+trashid.MustEncodeHashID(1), "0x7d273271690538cf855e5b3002a0dd8c154bb060", requestBodyBytes, map[string]string{
		"Content-Type": "application/json",
	})

	assert.Equal(t, 400, status)
	jsonAssert(t, body, map[string]any{
		"error": "Ticker is invalid",
	})
}

func TestV1CreateCoin_InvalidLogoUri(t *testing.T) {
	app := emptyTestApp(t)
	database.Seed(app.pool.Replicas[0], database.FixtureMap{
		"users": {
			{
				"user_id":     1,
				"wallet":      "0x7d273271690538cf855e5b3002a0dd8c154bb060",
				"is_verified": true,
			},
		},
	})

	requestBody := CreateCoinBody{
		Mint:        "bearR26zyyB3fNQm5wWv1ZfN8MPQDUMwaAuoG79b1Yj",
		Ticker:      "BEAR",
		Decimals:    9,
		Name:        "BEAR",
		LogoUri:     "not-a-valid-url", // Invalid URL format
		Description: "A majestic bear token",
	}
	requestBodyBytes, err := json.Marshal(requestBody)
	assert.NoError(t, err)

	status, body := testPostWithWallet(t, app, "/v1/coins?user_id="+trashid.MustEncodeHashID(1), "0x7d273271690538cf855e5b3002a0dd8c154bb060", requestBodyBytes, map[string]string{
		"Content-Type": "application/json",
	})

	assert.Equal(t, 400, status)
	jsonAssert(t, body, map[string]any{
		"error": "LogoUri is invalid",
	})
}
