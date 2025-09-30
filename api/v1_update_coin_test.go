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

	link1 := "https://x.com/bear_token"
	link2 := "https://instagram.com/bear_token"
	link3 := "https://tiktok.com/@bear_token"
	link4 := "https://bear-token.com"

	requestBody := UpdateCoinBody{
		Description: "Updated description for the bear token",
		Link1:       &link1,
		Link2:       &link2,
		Link3:       &link3,
		Link4:       &link4,
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
		"data.mint":        "bearR26zyyB3fNQm5wWv1ZfN8MPQDUMwaAuoG79b1Yj",
		"data.ticker":      "$BEAR",
		"data.name":        "BEAR",
		"data.description": "Updated description for the bear token",
		"data.link_1":      "https://x.com/bear_token",
		"data.link_2":      "https://instagram.com/bear_token",
		"data.link_3":      "https://tiktok.com/@bear_token",
		"data.link_4":      "https://bear-token.com",
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

	link1_2 := "https://test1.com"
	link2_2 := "https://test2.com"
	link3_2 := "https://test3.com"
	link4_2 := "https://test4.com"

	requestBody := UpdateCoinBody{
		Description: "Updated description",
		Link1:       &link1_2,
		Link2:       &link2_2,
		Link3:       &link3_2,
		Link4:       &link4_2,
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

	link1_3 := "https://test1_3.com"
	link2_3 := "https://test2_3.com"
	link3_3 := "https://test3_3.com"
	link4_3 := "https://test4_3.com"

	requestBody := UpdateCoinBody{
		Description: "Updated description",
		Link1:       &link1_3,
		Link2:       &link2_3,
		Link3:       &link3_3,
		Link4:       &link4_3,
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

	// Test updating only Link1
	link1_4 := "https://bear_token_handle.com"
	requestBody = UpdateCoinBody{
		Link1: &link1_4,
	}
	requestBodyBytes, err = json.Marshal(requestBody)
	assert.NoError(t, err)

	status, body = testPostWithWallet(t, app, "/v1/coins/bearR26zyyB3fNQm5wWv1ZfN8MPQDUMwaAuoG79b1Yj?user_id="+trashid.MustEncodeHashID(1), "0x7d273271690538cf855e5b3002a0dd8c154bb060", requestBodyBytes, nil)

	assert.Equal(t, 200, status)
	jsonAssert(t, body, map[string]any{
		"success": true,
	})

	// Test updating multiple fields at once
	link2_5 := "https://bear_token_insta.com"
	link3_5 := "https://bear_token_tiktok.com"
	link4_5 := "https://bear-token.com"

	requestBody = UpdateCoinBody{
		Link2: &link2_5,
		Link3: &link3_5,
		Link4: &link4_5,
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

	// Test invalid Link4 URL
	invalidLink4 := "definitely-not-a-url"
	requestBody := UpdateCoinBody{
		Link4: &invalidLink4,
	}
	requestBodyBytes, err := json.Marshal(requestBody)
	assert.NoError(t, err)

	status, _ := testPostWithWallet(t, app, "/v1/coins/bearR26zyyB3fNQm5wWv1ZfN8MPQDUMwaAuoG79b1Yj?user_id="+trashid.MustEncodeHashID(1), "0x7d273271690538cf855e5b3002a0dd8c154bb060", requestBodyBytes, nil)

	assert.Equal(t, 400, status)

	// Test valid URLs work
	validLink1 := "https://example1.com"
	validLink2 := "https://example2.com"
	validLink3 := "https://example3.com"
	validLink4 := "https://example4.com"

	requestBody = UpdateCoinBody{
		Link1: &validLink1,
		Link2: &validLink2,
		Link3: &validLink3,
		Link4: &validLink4,
	}
	requestBodyBytes, err = json.Marshal(requestBody)
	assert.NoError(t, err)

	status, body := testPostWithWallet(t, app, "/v1/coins/bearR26zyyB3fNQm5wWv1ZfN8MPQDUMwaAuoG79b1Yj?user_id="+trashid.MustEncodeHashID(1), "0x7d273271690538cf855e5b3002a0dd8c154bb060", requestBodyBytes, nil)

	assert.Equal(t, 200, status)
	jsonAssert(t, body, map[string]any{
		"success": true,
	})

	// Test deleting links by passing empty strings
	emptyString := ""
	requestBody = UpdateCoinBody{
		Link1: &emptyString,
		Link2: &emptyString,
		Link3: &emptyString,
		Link4: &emptyString,
	}
	requestBodyBytes, err = json.Marshal(requestBody)
	assert.NoError(t, err)

	status, body = testPostWithWallet(t, app, "/v1/coins/bearR26zyyB3fNQm5wWv1ZfN8MPQDUMwaAuoG79b1Yj?user_id="+trashid.MustEncodeHashID(1), "0x7d273271690538cf855e5b3002a0dd8c154bb060", requestBodyBytes, nil)

	assert.Equal(t, 200, status)
	jsonAssert(t, body, map[string]any{
		"data.link_1": nil,
		"data.link_2": nil,
		"data.link_3": nil,
		"data.link_4": nil,
	})
}

func TestV1UpdateCoin_DeleteFields(t *testing.T) {
	// Test deleting link_1 only
	t.Run("delete link_1", func(t *testing.T) {
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
					"link_1":      "https://original1.com",
					"link_2":      "https://original2.com",
					"link_3":      "https://original3.com",
					"link_4":      "https://original4.com",
				},
			},
		})

		emptyString := ""
		requestBody := UpdateCoinBody{
			Link1: &emptyString,
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
			"data.link_1": nil,
			"data.link_2": "https://original2.com",
			"data.link_3": "https://original3.com",
			"data.link_4": "https://original4.com",
		})
	})

	// Test deleting link_2 only
	t.Run("delete link_2", func(t *testing.T) {
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
					"link_1":      "https://original1.com",
					"link_2":      "https://original2.com",
					"link_3":      "https://original3.com",
					"link_4":      "https://original4.com",
				},
			},
		})

		emptyString := ""
		requestBody := UpdateCoinBody{
			Link2: &emptyString,
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
			"data.link_1": "https://original1.com",
			"data.link_2": nil,
			"data.link_3": "https://original3.com",
			"data.link_4": "https://original4.com",
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
					"mint":        "bearR26zyyB3fNQm5wWv1ZfN8MPQDUMwaAuoG79b1Yj",
					"ticker":      "$BEAR",
					"user_id":     1,
					"decimals":    9,
					"name":        "BEAR",
					"description": "Original description",
					"link_1":      "https://original1.com",
					"link_2":      "https://original2.com",
					"link_3":      "https://original3.com",
					"link_4":      "https://original4.com",
				},
			},
		})

		emptyString := ""
		requestBody := UpdateCoinBody{
			Link1: &emptyString,
			Link2: &emptyString,
			Link3: &emptyString,
			Link4: &emptyString,
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
			"data.link_1": nil,
			"data.link_2": nil,
			"data.link_3": nil,
			"data.link_4": nil,
		})
	})
}
