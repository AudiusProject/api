package api

import (
	"encoding/json"
	"strings"
	"testing"

	"api.audius.co/database"
	"api.audius.co/trashid"
	"github.com/stretchr/testify/assert"
)

func TestV1UpdateCoin(t *testing.T) {
	app := emptyTestApp(t)
	database.Seed(app.writePool, database.FixtureMap{
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

	xHandle := "bear_token"
	instagramHandle := "bear_token"
	tiktokHandle := "bear_token"
	website := "https://bear-token.com"

	requestBody := UpdateCoinBody{
		Description:     "Updated description for the bear token",
		XHandle:         &xHandle,
		InstagramHandle: &instagramHandle,
		TiktokHandle:    &tiktokHandle,
		Website:         &website,
	}
	requestBodyBytes, err := json.Marshal(requestBody)
	assert.NoError(t, err)

	status, body := testPostWithWallet(t, app, "/v1/coins/bearR26zyyB3fNQm5wWv1ZfN8MPQDUMwaAuoG79b1Yj?user_id="+trashid.MustEncodeHashID(1), "0x7d273271690538cf855e5b3002a0dd8c154bb060", requestBodyBytes, nil)

	assert.Equal(t, 200, status)
	jsonAssert(t, body, map[string]any{
		"success": true,
	})

	// Verify the coin was actually updated by fetching it via API
	status, body = testGet(t, app, "/v1/coins/bearR26zyyB3fNQm5wWv1ZfN8MPQDUMwaAuoG79b1Yj")
	assert.Equal(t, 200, status)
	jsonAssert(t, body, map[string]any{
		"data.mint":             "bearR26zyyB3fNQm5wWv1ZfN8MPQDUMwaAuoG79b1Yj",
		"data.ticker":           "$BEAR",
		"data.name":             "BEAR",
		"data.description":      "Updated description for the bear token",
		"data.x_handle":         "bear_token",
		"data.instagram_handle": "bear_token",
		"data.tiktok_handle":    "bear_token",
		"data.website":          "https://bear-token.com",
	})
}

func TestV1UpdateCoin_CoinNotFound(t *testing.T) {
	app := emptyTestApp(t)
	database.Seed(app.writePool, database.FixtureMap{
		"users": {
			{
				"user_id":     1,
				"wallet":      "0x7d273271690538cf855e5b3002a0dd8c154bb060",
				"is_verified": true,
			},
		},
	})

	xHandle2 := "test_handle"
	instagramHandle2 := "test_handle"
	tiktokHandle2 := "test_handle"
	website2 := "https://test.com"

	requestBody := UpdateCoinBody{
		Description:     "Updated description",
		XHandle:         &xHandle2,
		InstagramHandle: &instagramHandle2,
		TiktokHandle:    &tiktokHandle2,
		Website:         &website2,
	}
	requestBodyBytes, err := json.Marshal(requestBody)
	assert.NoError(t, err)

	status, body := testPostWithWallet(t, app, "/v1/coins/nonexistentMint?user_id="+trashid.MustEncodeHashID(1), "0x7d273271690538cf855e5b3002a0dd8c154bb060", requestBodyBytes, nil)

	assert.Equal(t, 404, status)
	jsonAssert(t, body, map[string]any{
		"error": "Coin not found",
	})
}

func TestV1UpdateCoin_Unauthorized(t *testing.T) {
	app := emptyTestApp(t)
	database.Seed(app.writePool, database.FixtureMap{
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

	xHandle3 := "test_handle_3"
	instagramHandle3 := "test_handle_3"
	tiktokHandle3 := "test_handle_3"
	website3 := "https://test.com"

	requestBody := UpdateCoinBody{
		Description:     "Updated description",
		XHandle:         &xHandle3,
		InstagramHandle: &instagramHandle3,
		TiktokHandle:    &tiktokHandle3,
		Website:         &website3,
	}
	requestBodyBytes, err := json.Marshal(requestBody)
	assert.NoError(t, err)

	// Try to update with user 2 (who doesn't own the coin)
	status, body := testPostWithWallet(t, app, "/v1/coins/bearR26zyyB3fNQm5wWv1ZfN8MPQDUMwaAuoG79b1Yj?user_id="+trashid.MustEncodeHashID(2), "0xc3d1d41e6872ffbd15c473d14fc3a9250be5b5e0", requestBodyBytes, nil)

	assert.Equal(t, 403, status)
	jsonAssert(t, body, map[string]any{
		"error": "You do not own this coin",
	})
}

func TestV1UpdateCoin_Validation(t *testing.T) {
	app := emptyTestApp(t)
	database.Seed(app.writePool, database.FixtureMap{
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
	longDescription := strings.Repeat("a", 2501)

	requestBody := UpdateCoinBody{
		Description: longDescription,
	}
	requestBodyBytes, err := json.Marshal(requestBody)
	assert.NoError(t, err)

	status, _ := testPostWithWallet(t, app, "/v1/coins/bearR26zyyB3fNQm5wWv1ZfN8MPQDUMwaAuoG79b1Yj?user_id="+trashid.MustEncodeHashID(1), "0x7d273271690538cf855e5b3002a0dd8c154bb060", requestBodyBytes, nil)

	assert.Equal(t, 400, status)
}

func TestV1UpdateCoin_IndividualFields(t *testing.T) {
	app := emptyTestApp(t)
	database.Seed(app.writePool, database.FixtureMap{
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

	status, body := testPostWithWallet(t, app, "/v1/coins/bearR26zyyB3fNQm5wWv1ZfN8MPQDUMwaAuoG79b1Yj?user_id="+trashid.MustEncodeHashID(1), "0x7d273271690538cf855e5b3002a0dd8c154bb060", requestBodyBytes, nil)

	assert.Equal(t, 200, status)
	jsonAssert(t, body, map[string]any{
		"success": true,
	})

	// Test updating only Twitter
	xHandle4 := "bear_token_handle"
	requestBody = UpdateCoinBody{
		XHandle: &xHandle4,
	}
	requestBodyBytes, err = json.Marshal(requestBody)
	assert.NoError(t, err)

	status, body = testPostWithWallet(t, app, "/v1/coins/bearR26zyyB3fNQm5wWv1ZfN8MPQDUMwaAuoG79b1Yj?user_id="+trashid.MustEncodeHashID(1), "0x7d273271690538cf855e5b3002a0dd8c154bb060", requestBodyBytes, nil)

	assert.Equal(t, 200, status)
	jsonAssert(t, body, map[string]any{
		"success": true,
	})

	// Test updating multiple fields at once
	instagramHandle5 := "bear_token_insta"
	tiktokHandle5 := "bear_token_tiktok"
	website5 := "https://bear-token.com"

	requestBody = UpdateCoinBody{
		InstagramHandle: &instagramHandle5,
		TiktokHandle:    &tiktokHandle5,
		Website:         &website5,
	}
	requestBodyBytes, err = json.Marshal(requestBody)
	assert.NoError(t, err)

	status, body = testPostWithWallet(t, app, "/v1/coins/bearR26zyyB3fNQm5wWv1ZfN8MPQDUMwaAuoG79b1Yj?user_id="+trashid.MustEncodeHashID(1), "0x7d273271690538cf855e5b3002a0dd8c154bb060", requestBodyBytes, nil)

	assert.Equal(t, 200, status)
	jsonAssert(t, body, map[string]any{
		"success": true,
	})
}

func TestV1UpdateCoin_NoFields(t *testing.T) {
	app := emptyTestApp(t)
	database.Seed(app.writePool, database.FixtureMap{
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

	// Test updating with no fields provided (empty request body) - should fail
	requestBody := UpdateCoinBody{}
	requestBodyBytes, err := json.Marshal(requestBody)
	assert.NoError(t, err)

	status, body := testPostWithWallet(t, app, "/v1/coins/bearR26zyyB3fNQm5wWv1ZfN8MPQDUMwaAuoG79b1Yj?user_id="+trashid.MustEncodeHashID(1), "0x7d273271690538cf855e5b3002a0dd8c154bb060", requestBodyBytes, nil)

	assert.Equal(t, 400, status)
	jsonAssert(t, body, map[string]any{
		"error": "At least one field must be provided for update",
	})
}

func TestV1UpdateCoin_URLValidation(t *testing.T) {
	app := emptyTestApp(t)
	database.Seed(app.writePool, database.FixtureMap{
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

	// Test invalid Website URL
	invalidWebsite := "definitely-not-a-url"
	requestBody := UpdateCoinBody{
		Website: &invalidWebsite,
	}
	requestBodyBytes, err := json.Marshal(requestBody)
	assert.NoError(t, err)

	status, _ := testPostWithWallet(t, app, "/v1/coins/bearR26zyyB3fNQm5wWv1ZfN8MPQDUMwaAuoG79b1Yj?user_id="+trashid.MustEncodeHashID(1), "0x7d273271690538cf855e5b3002a0dd8c154bb060", requestBodyBytes, nil)

	assert.Equal(t, 400, status)

	// Test valid URLs work
	validXHandle := "example_handle"
	validInstagramHandle := "example_handle"
	validTiktokHandle := "example_handle"
	validWebsite := "https://example.com"

	requestBody = UpdateCoinBody{
		XHandle:         &validXHandle,
		InstagramHandle: &validInstagramHandle,
		TiktokHandle:    &validTiktokHandle,
		Website:         &validWebsite,
	}
	requestBodyBytes, err = json.Marshal(requestBody)
	assert.NoError(t, err)

	status, body := testPostWithWallet(t, app, "/v1/coins/bearR26zyyB3fNQm5wWv1ZfN8MPQDUMwaAuoG79b1Yj?user_id="+trashid.MustEncodeHashID(1), "0x7d273271690538cf855e5b3002a0dd8c154bb060", requestBodyBytes, nil)

	assert.Equal(t, 200, status)
	jsonAssert(t, body, map[string]any{
		"success": true,
	})

	// Test deleting handles by passing empty strings
	emptyString := ""
	requestBody = UpdateCoinBody{
		XHandle:         &emptyString,
		InstagramHandle: &emptyString,
		TiktokHandle:    &emptyString,
		Website:         &emptyString,
	}
	requestBodyBytes, err = json.Marshal(requestBody)
	assert.NoError(t, err)

	status, body = testPostWithWallet(t, app, "/v1/coins/bearR26zyyB3fNQm5wWv1ZfN8MPQDUMwaAuoG79b1Yj?user_id="+trashid.MustEncodeHashID(1), "0x7d273271690538cf855e5b3002a0dd8c154bb060", requestBodyBytes, nil)

	assert.Equal(t, 200, status)
	jsonAssert(t, body, map[string]any{
		"data.x_handle":         nil,
		"data.instagram_handle": nil,
		"data.tiktok_handle":    nil,
		"data.website":          nil,
	})
}

func TestV1UpdateCoin_DeleteFields(t *testing.T) {
	// Test deleting x_handle only
	t.Run("delete x_handle", func(t *testing.T) {
		app := emptyTestApp(t)
		database.Seed(app.writePool, database.FixtureMap{
			"users": {
				{
					"user_id":     1,
					"wallet":      "0x7d273271690538cf855e5b3002a0dd8c154bb060",
					"is_verified": true,
				},
			},
			"artist_coins": {
				{
					"mint":             "bearR26zyyB3fNQm5wWv1ZfN8MPQDUMwaAuoG79b1Yj",
					"ticker":           "$BEAR",
					"user_id":          1,
					"decimals":         9,
					"name":             "BEAR",
					"description":      "Original description",
					"x_handle":         "original_handle",
					"instagram_handle": "original_handle",
					"tiktok_handle":    "original_handle",
					"website":          "https://original.com",
				},
			},
		})

		emptyString := ""
		requestBody := UpdateCoinBody{
			XHandle: &emptyString,
		}
		requestBodyBytes, err := json.Marshal(requestBody)
		assert.NoError(t, err)

		status, body := testPostWithWallet(t, app, "/v1/coins/bearR26zyyB3fNQm5wWv1ZfN8MPQDUMwaAuoG79b1Yj?user_id="+trashid.MustEncodeHashID(1), "0x7d273271690538cf855e5b3002a0dd8c154bb060", requestBodyBytes, nil)

		assert.Equal(t, 200, status)
		jsonAssert(t, body, map[string]any{
			"success": true,
		})

		// Verify the deletion via GET
		status, body = testGet(t, app, "/v1/coins/bearR26zyyB3fNQm5wWv1ZfN8MPQDUMwaAuoG79b1Yj")
		assert.Equal(t, 200, status)
		jsonAssert(t, body, map[string]any{
			"data.x_handle":         nil,
			"data.instagram_handle": "original_handle",
			"data.tiktok_handle":    "original_handle",
			"data.website":          "https://original.com",
		})
	})

	// Test deleting instagram_handle only
	t.Run("delete instagram_handle", func(t *testing.T) {
		app := emptyTestApp(t)
		database.Seed(app.writePool, database.FixtureMap{
			"users": {
				{
					"user_id":     1,
					"wallet":      "0x7d273271690538cf855e5b3002a0dd8c154bb060",
					"is_verified": true,
				},
			},
			"artist_coins": {
				{
					"mint":             "bearR26zyyB3fNQm5wWv1ZfN8MPQDUMwaAuoG79b1Yj",
					"ticker":           "$BEAR",
					"user_id":          1,
					"decimals":         9,
					"name":             "BEAR",
					"description":      "Original description",
					"x_handle":         "original_handle",
					"instagram_handle": "original_handle",
					"tiktok_handle":    "original_handle",
					"website":          "https://original.com",
				},
			},
		})

		emptyString := ""
		requestBody := UpdateCoinBody{
			InstagramHandle: &emptyString,
		}
		requestBodyBytes, err := json.Marshal(requestBody)
		assert.NoError(t, err)

		status, body := testPostWithWallet(t, app, "/v1/coins/bearR26zyyB3fNQm5wWv1ZfN8MPQDUMwaAuoG79b1Yj?user_id="+trashid.MustEncodeHashID(1), "0x7d273271690538cf855e5b3002a0dd8c154bb060", requestBodyBytes, nil)

		assert.Equal(t, 200, status)
		jsonAssert(t, body, map[string]any{
			"success": true,
		})

		// Verify the deletion via GET
		status, body = testGet(t, app, "/v1/coins/bearR26zyyB3fNQm5wWv1ZfN8MPQDUMwaAuoG79b1Yj")
		assert.Equal(t, 200, status)
		jsonAssert(t, body, map[string]any{
			"data.x_handle":         "original_handle",
			"data.instagram_handle": nil,
			"data.tiktok_handle":    "original_handle",
			"data.website":          "https://original.com",
		})
	})

	// Test deleting all handles
	t.Run("delete all handles", func(t *testing.T) {
		app := emptyTestApp(t)
		database.Seed(app.writePool, database.FixtureMap{
			"users": {
				{
					"user_id":     1,
					"wallet":      "0x7d273271690538cf855e5b3002a0dd8c154bb060",
					"is_verified": true,
				},
			},
			"artist_coins": {
				{
					"mint":             "bearR26zyyB3fNQm5wWv1ZfN8MPQDUMwaAuoG79b1Yj",
					"ticker":           "$BEAR",
					"user_id":          1,
					"decimals":         9,
					"name":             "BEAR",
					"description":      "Original description",
					"x_handle":         "original_handle",
					"instagram_handle": "original_handle",
					"tiktok_handle":    "original_handle",
					"website":          "https://original.com",
				},
			},
		})

		emptyString := ""
		requestBody := UpdateCoinBody{
			XHandle:         &emptyString,
			InstagramHandle: &emptyString,
			TiktokHandle:    &emptyString,
			Website:         &emptyString,
		}
		requestBodyBytes, err := json.Marshal(requestBody)
		assert.NoError(t, err)

		status, body := testPostWithWallet(t, app, "/v1/coins/bearR26zyyB3fNQm5wWv1ZfN8MPQDUMwaAuoG79b1Yj?user_id="+trashid.MustEncodeHashID(1), "0x7d273271690538cf855e5b3002a0dd8c154bb060", requestBodyBytes, nil)

		assert.Equal(t, 200, status)
		jsonAssert(t, body, map[string]any{
			"success": true,
		})

		// Verify all deletions via GET
		status, body = testGet(t, app, "/v1/coins/bearR26zyyB3fNQm5wWv1ZfN8MPQDUMwaAuoG79b1Yj")
		assert.Equal(t, 200, status)
		jsonAssert(t, body, map[string]any{
			"data.x_handle":         nil,
			"data.instagram_handle": nil,
			"data.tiktok_handle":    nil,
			"data.website":          nil,
		})
	})
}
