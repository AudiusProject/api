package api

import (
	"encoding/json"
	"testing"

	"api.audius.co/database"
	"api.audius.co/trashid"
	"github.com/stretchr/testify/assert"
)

func TestV1UpdateCoin(t *testing.T) {
	app := emptyTestApp(t)
	database.Seed(app.pool.Replicas[0], database.FixtureMap{
		"users": {
			{
				"user_id":     1,
				"wallet":      "0x7d273271690538cf855e5b3002a0dd8c154bb060",
				"is_verified": true,
			},
		},
		"artist_coins": {
			{
				"mint":        "bearR26zyyB3fNQm5wWv1ZfN8MPQDUMwaAuoG79b1Yj",
				"ticker":      "$BEAR",
				"user_id":     1,
				"decimals":    9,
				"name":        "BEAR",
				"logo_uri":    "https://example.com/bear-logo.png",
				"description": "Original description",
			},
		},
	})

	requestBody := UpdateCoinBody{
		Description:     "Updated description for the bear token",
		XHandle:         "https://x.com/bear_token",
		InstagramHandle: "https://instagram.com/bear_token",
		TiktokHandle:    "https://tiktok.com/@bear_token",
		Website:         "https://bear-token.com",
	}
	requestBodyBytes, err := json.Marshal(requestBody)
	assert.NoError(t, err)

	status, body := testPostWithWallet(t, app, "/v1/coins/bearR26zyyB3fNQm5wWv1ZfN8MPQDUMwaAuoG79b1Yj?user_id="+trashid.MustEncodeHashID(1), "0x7d273271690538cf855e5b3002a0dd8c154bb060", requestBodyBytes, map[string]string{
		"Content-Type": "application/json",
	})

	assert.Equal(t, 200, status)
	jsonAssert(t, body, map[string]any{
		"data.mint":             "bearR26zyyB3fNQm5wWv1ZfN8MPQDUMwaAuoG79b1Yj",
		"data.ticker":           "$BEAR",
		"data.user_id":          1,
		"data.decimals":         9,
		"data.name":             "BEAR",
		"data.logo_uri":         "https://example.com/bear-logo.png",
		"data.description":      "Updated description for the bear token",
		"data.x_handle":         "https://x.com/bear_token",
		"data.instagram_handle": "https://instagram.com/bear_token",
		"data.tiktok_handle":    "https://tiktok.com/@bear_token",
		"data.website":          "https://bear-token.com",
	})

	// Verify the coin was actually updated by fetching it via API
	status, body = testGet(t, app, "/v1/coins/bearR26zyyB3fNQm5wWv1ZfN8MPQDUMwaAuoG79b1Yj")
	assert.Equal(t, 200, status)
	jsonAssert(t, body, map[string]any{
		"data.mint":             "bearR26zyyB3fNQm5wWv1ZfN8MPQDUMwaAuoG79b1Yj",
		"data.ticker":           "$BEAR",
		"data.name":             "BEAR",
		"data.description":      "Updated description for the bear token",
		"data.x_handle":         "https://x.com/bear_token",
		"data.instagram_handle": "https://instagram.com/bear_token",
		"data.tiktok_handle":    "https://tiktok.com/@bear_token",
		"data.website":          "https://bear-token.com",
	})
}

func TestV1UpdateCoin_CoinNotFound(t *testing.T) {
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

	requestBody := UpdateCoinBody{
		Description:     "Updated description",
		XHandle:         "https://x.com/test",
		InstagramHandle: "https://instagram.com/test",
		TiktokHandle:    "https://tiktok.com/@test",
		Website:         "https://test.com",
	}
	requestBodyBytes, err := json.Marshal(requestBody)
	assert.NoError(t, err)

	status, body := testPostWithWallet(t, app, "/v1/coins/nonexistentMint?user_id="+trashid.MustEncodeHashID(1), "0x7d273271690538cf855e5b3002a0dd8c154bb060", requestBodyBytes, map[string]string{
		"Content-Type": "application/json",
	})

	assert.Equal(t, 404, status)
	jsonAssert(t, body, map[string]any{
		"error": "Coin not found",
	})
}

func TestV1UpdateCoin_Unauthorized(t *testing.T) {
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
		"artist_coins": {
			{
				"mint":        "bearR26zyyB3fNQm5wWv1ZfN8MPQDUMwaAuoG79b1Yj",
				"ticker":      "$BEAR",
				"user_id":     1, // Owned by user 1
				"decimals":    9,
				"name":        "BEAR",
				"description": "Original description",
			},
		},
	})

	requestBody := UpdateCoinBody{
		Description:     "Updated description",
		XHandle:         "https://x.com/test",
		InstagramHandle: "https://instagram.com/test",
		TiktokHandle:    "https://tiktok.com/@test",
		Website:         "https://test.com",
	}
	requestBodyBytes, err := json.Marshal(requestBody)
	assert.NoError(t, err)

	// Try to update with user 2 (who doesn't own the coin)
	status, body := testPostWithWallet(t, app, "/v1/coins/bearR26zyyB3fNQm5wWv1ZfN8MPQDUMwaAuoG79b1Yj?user_id="+trashid.MustEncodeHashID(2), "0xc3d1d41e6872ffbd15c473d14fc3a9250be5b5e0", requestBodyBytes, map[string]string{
		"Content-Type": "application/json",
	})

	assert.Equal(t, 403, status)
	jsonAssert(t, body, map[string]any{
		"error": "You do not own this coin",
	})
}

func TestV1UpdateCoin_Validation(t *testing.T) {
	app := emptyTestApp(t)
	database.Seed(app.pool.Replicas[0], database.FixtureMap{
		"users": {
			{
				"user_id":     1,
				"wallet":      "0x7d273271690538cf855e5b3002a0dd8c154bb060",
				"is_verified": true,
			},
		},
		"artist_coins": {
			{
				"mint":        "bearR26zyyB3fNQm5wWv1ZfN8MPQDUMwaAuoG79b1Yj",
				"ticker":      "$BEAR",
				"user_id":     1,
				"decimals":    9,
				"name":        "BEAR",
				"description": "Original description",
			},
		},
	})

	// Test with description that's too long (>2500 chars)
	longDescription := ""
	for len(longDescription) <= 2500 {
		longDescription += "a"
	}

	requestBody := UpdateCoinBody{
		Description: longDescription,
	}
	requestBodyBytes, err := json.Marshal(requestBody)
	assert.NoError(t, err)

	status, _ := testPostWithWallet(t, app, "/v1/coins/bearR26zyyB3fNQm5wWv1ZfN8MPQDUMwaAuoG79b1Yj?user_id="+trashid.MustEncodeHashID(1), "0x7d273271690538cf855e5b3002a0dd8c154bb060", requestBodyBytes, map[string]string{
		"Content-Type": "application/json",
	})

	assert.Equal(t, 400, status)
	// The validation error will be handled by the ParseAndValidateBody method
}

func TestV1UpdateCoin_IndividualFields(t *testing.T) {
	app := emptyTestApp(t)
	database.Seed(app.pool.Replicas[0], database.FixtureMap{
		"users": {
			{
				"user_id":     1,
				"wallet":      "0x7d273271690538cf855e5b3002a0dd8c154bb060",
				"is_verified": true,
			},
		},
		"artist_coins": {
			{
				"mint":        "bearR26zyyB3fNQm5wWv1ZfN8MPQDUMwaAuoG79b1Yj",
				"ticker":      "$BEAR",
				"user_id":     1,
				"decimals":    9,
				"name":        "BEAR",
				"description": "Original description",
			},
		},
	})

	// Test updating only description
	requestBody := UpdateCoinBody{
		Description: "Updated description only",
	}
	requestBodyBytes, err := json.Marshal(requestBody)
	assert.NoError(t, err)

	status, body := testPostWithWallet(t, app, "/v1/coins/bearR26zyyB3fNQm5wWv1ZfN8MPQDUMwaAuoG79b1Yj?user_id="+trashid.MustEncodeHashID(1), "0x7d273271690538cf855e5b3002a0dd8c154bb060", requestBodyBytes, map[string]string{
		"Content-Type": "application/json",
	})

	assert.Equal(t, 200, status)
	jsonAssert(t, body, map[string]any{
		"data.description":      "Updated description only",
		"data.x_handle":         "",
		"data.instagram_handle": "",
		"data.tiktok_handle":    "",
		"data.website":          "",
	})

	// Test updating only Twitter
	requestBody = UpdateCoinBody{
		XHandle: "https://x.com/bear_token",
	}
	requestBodyBytes, err = json.Marshal(requestBody)
	assert.NoError(t, err)

	status, body = testPostWithWallet(t, app, "/v1/coins/bearR26zyyB3fNQm5wWv1ZfN8MPQDUMwaAuoG79b1Yj?user_id="+trashid.MustEncodeHashID(1), "0x7d273271690538cf855e5b3002a0dd8c154bb060", requestBodyBytes, map[string]string{
		"Content-Type": "application/json",
	})

	assert.Equal(t, 200, status)
	jsonAssert(t, body, map[string]any{
		"data.description":      "Updated description only",
		"data.x_handle":         "https://x.com/bear_token",
		"data.instagram_handle": "",
		"data.tiktok_handle":    "",
		"data.website":          "",
	})

	// Test updating multiple fields at once
	requestBody = UpdateCoinBody{
		InstagramHandle: "https://instagram.com/bear_token",
		TiktokHandle:    "https://tiktok.com/@bear_token",
		Website:         "https://bear-token.com",
	}
	requestBodyBytes, err = json.Marshal(requestBody)
	assert.NoError(t, err)

	status, body = testPostWithWallet(t, app, "/v1/coins/bearR26zyyB3fNQm5wWv1ZfN8MPQDUMwaAuoG79b1Yj?user_id="+trashid.MustEncodeHashID(1), "0x7d273271690538cf855e5b3002a0dd8c154bb060", requestBodyBytes, map[string]string{
		"Content-Type": "application/json",
	})

	assert.Equal(t, 200, status)
	jsonAssert(t, body, map[string]any{
		"data.description":      "Updated description only",
		"data.x_handle":         "https://x.com/bear_token",
		"data.instagram_handle": "https://instagram.com/bear_token",
		"data.tiktok_handle":    "https://tiktok.com/@bear_token",
		"data.website":          "https://bear-token.com",
	})
}

func TestV1UpdateCoin_URLValidation(t *testing.T) {
	app := emptyTestApp(t)
	database.Seed(app.pool.Replicas[0], database.FixtureMap{
		"users": {
			{
				"user_id":     1,
				"wallet":      "0x7d273271690538cf855e5b3002a0dd8c154bb060",
				"is_verified": true,
			},
		},
		"artist_coins": {
			{
				"mint":        "bearR26zyyB3fNQm5wWv1ZfN8MPQDUMwaAuoG79b1Yj",
				"ticker":      "$BEAR",
				"user_id":     1,
				"decimals":    9,
				"name":        "BEAR",
				"description": "Original description",
			},
		},
	})

	// Test invalid Twitter URL
	requestBody := UpdateCoinBody{
		XHandle: "not-a-valid-url",
	}
	requestBodyBytes, err := json.Marshal(requestBody)
	assert.NoError(t, err)

	status, _ := testPostWithWallet(t, app, "/v1/coins/bearR26zyyB3fNQm5wWv1ZfN8MPQDUMwaAuoG79b1Yj?user_id="+trashid.MustEncodeHashID(1), "0x7d273271690538cf855e5b3002a0dd8c154bb060", requestBodyBytes, map[string]string{
		"Content-Type": "application/json",
	})

	assert.Equal(t, 400, status)

	// Test invalid Instagram URL
	requestBody = UpdateCoinBody{
		InstagramHandle: "also-not-valid",
	}
	requestBodyBytes, err = json.Marshal(requestBody)
	assert.NoError(t, err)

	status, _ = testPostWithWallet(t, app, "/v1/coins/bearR26zyyB3fNQm5wWv1ZfN8MPQDUMwaAuoG79b1Yj?user_id="+trashid.MustEncodeHashID(1), "0x7d273271690538cf855e5b3002a0dd8c154bb060", requestBodyBytes, map[string]string{
		"Content-Type": "application/json",
	})

	assert.Equal(t, 400, status)

	// Test invalid TikTok URL
	requestBody = UpdateCoinBody{
		TiktokHandle: "invalid-url",
	}
	requestBodyBytes, err = json.Marshal(requestBody)
	assert.NoError(t, err)

	status, _ = testPostWithWallet(t, app, "/v1/coins/bearR26zyyB3fNQm5wWv1ZfN8MPQDUMwaAuoG79b1Yj?user_id="+trashid.MustEncodeHashID(1), "0x7d273271690538cf855e5b3002a0dd8c154bb060", requestBodyBytes, map[string]string{
		"Content-Type": "application/json",
	})

	assert.Equal(t, 400, status)

	// Test invalid Website URL
	requestBody = UpdateCoinBody{
		Website: "definitely-not-a-url",
	}
	requestBodyBytes, err = json.Marshal(requestBody)
	assert.NoError(t, err)

	status, _ = testPostWithWallet(t, app, "/v1/coins/bearR26zyyB3fNQm5wWv1ZfN8MPQDUMwaAuoG79b1Yj?user_id="+trashid.MustEncodeHashID(1), "0x7d273271690538cf855e5b3002a0dd8c154bb060", requestBodyBytes, map[string]string{
		"Content-Type": "application/json",
	})

	assert.Equal(t, 400, status)

	// Test valid URLs work
	requestBody = UpdateCoinBody{
		XHandle:         "https://x.com/example",
		InstagramHandle: "https://www.instagram.com/example",
		TiktokHandle:    "https://www.tiktok.com/@example",
		Website:         "https://example.com",
	}
	requestBodyBytes, err = json.Marshal(requestBody)
	assert.NoError(t, err)

	status, body := testPostWithWallet(t, app, "/v1/coins/bearR26zyyB3fNQm5wWv1ZfN8MPQDUMwaAuoG79b1Yj?user_id="+trashid.MustEncodeHashID(1), "0x7d273271690538cf855e5b3002a0dd8c154bb060", requestBodyBytes, map[string]string{
		"Content-Type": "application/json",
	})

	assert.Equal(t, 200, status)
	jsonAssert(t, body, map[string]any{
		"data.x_handle":         "https://x.com/example",
		"data.instagram_handle": "https://www.instagram.com/example",
		"data.tiktok_handle":    "https://www.tiktok.com/@example",
		"data.website":          "https://example.com",
	})
}
