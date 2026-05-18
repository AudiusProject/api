package api

import (
	"testing"
	"time"

	"api.audius.co/database"
	"github.com/stretchr/testify/assert"
)

func TestV1UsersPurchasesDownload(t *testing.T) {
	user1Wallet := "0x7d273271690538cf855e5b3002a0dd8c154bb060"
	user2Wallet := "0xc3d1d41e6872ffbd15c473d14fc3a9250be5b5e0"
	app := emptyTestApp(t)

	fixtures := database.FixtureMap{
		"users": []map[string]any{
			{"user_id": 1, "handle": "seller", "name": "seller", "wallet": user1Wallet, "spl_usdc_payout_wallet": user1Wallet},
			{"user_id": 2, "handle": "buyer1", "name": "buyer1", "wallet": user2Wallet},
			{"user_id": 3, "handle": "buyer2", "name": "buyer2"},
		},
		"tracks": []map[string]any{
			{"track_id": 1, "title": "track1", "owner_id": 1},
		},
		"track_routes": []map[string]any{
			{
				"track_id":     1,
				"owner_id":     1,
				"slug":         "track1",
				"title_slug":   "track1",
				"collision_id": 0,
			},
		},
		"sol_purchases": []map[string]any{
			{
				"signature":         "def",
				"instruction_index": 0,
				"buyer_user_id":     2,
				"country":           "US",
				"content_type":      "track",
				"content_id":        1,
				"amount":            1000000,
				"created_at":        time.Date(2024, 6, 4, 0, 0, 0, 0, time.UTC),
				"is_valid":          true,
			},
		},
		"sol_payments": []map[string]any{
			{"signature": "def", "instruction_index": 0, "route_index": 0, "to_account": user1Wallet, "amount": 900000, "slot": 101},
			{"signature": "def", "instruction_index": 0, "route_index": 1, "to_account": app.solanaConfig.StakingBridgeUsdcTokenAccount.String(), "amount": 100000, "slot": 101},
		},
		// Drives extra_amount = 500000 (amount=1000000 - base_price=50_cents*10000=500000)
		"track_price_history": []map[string]any{
			{"track_id": 1, "total_price_cents": 50, "block_timestamp": time.Date(2024, 6, 3, 0, 0, 0, 0, time.UTC), "splits": "[]"},
		},
	}

	database.Seed(app.writePool, fixtures)

	// json
	{
		status, body := testGetWithWallet(t, app, "/v1/users/ML51L/purchases/download/json", user2Wallet)
		assert.Equal(t, 200, status)

		jsonAssert(t, body, map[string]any{"data.#": 1})
		jsonAssert(t, body, map[string]any{"data.0.title": "track1"})
		jsonAssert(t, body, map[string]any{"data.0.link": "http://localhost:1323/seller/track1"})
		jsonAssert(t, body, map[string]any{"data.0.seller_name": "seller"})
		jsonAssert(t, body, map[string]any{"data.0.seller_user_id": 1})
		jsonAssert(t, body, map[string]any{"data.0.date": "2024-06-04T00:00:00Z"})
		jsonAssert(t, body, map[string]any{"data.0.sale_price": "1"})
		jsonAssert(t, body, map[string]any{"data.0.paid_to_artist": "0.9"})
		jsonAssert(t, body, map[string]any{"data.0.network_fee": "0.1"})
		jsonAssert(t, body, map[string]any{"data.0.pay_extra": "0.5"})
		jsonAssert(t, body, map[string]any{"data.0.total": "1.5"})
	}

	// csv
	{
		status, body := testGetWithWallet(t, app, "/v1/users/ML51L/purchases/download", user2Wallet)
		assert.Equal(t, 200, status)

		headers, dataRows, err := parseCSVLines(string(body))
		assert.NoError(t, err)

		// Verify headers
		expectedHeaders := []string{
			"title",
			"link",
			"artist",
			"date",
			"paid to artist",
			"network fee",
			"pay extra",
			"total",
		}
		assert.Equal(t, expectedHeaders, headers)

		assert.Equal(t, 1, len(dataRows))
		row := dataRows[0]
		assert.Equal(t, "track1", row[0])                              // title
		assert.Equal(t, "http://localhost:1323/seller/track1", row[1]) // link
		assert.Equal(t, "seller", row[2])                              // artist
		assert.Equal(t, "2024-06-04T00:00:00Z", row[3])                // date
		assert.Equal(t, "0.900000", row[4])                            // paid to artist
		assert.Equal(t, "0.100000", row[5])                            // network fee
		assert.Equal(t, "0.500000", row[6])                            // pay extra
		assert.Equal(t, "1.500000", row[7])                            // total
	}

	// Test 403 with no wallet
	{
		status, _ := testGet(t, app, "/v1/users/ML51L/purchases/download/json")
		assert.Equal(t, 403, status)
	}

	// Test 403 with unauthorized wallet
	{
		unauthorizedWallet := "0x855d28d495ec1b06364bb7a521212753e2190b95" // wallet without grants
		status, _ := testGetWithWallet(t, app, "/v1/users/ML51L/purchases/download/json", unauthorizedWallet)
		assert.Equal(t, 403, status)
	}
}

func TestV1UsersPurchasesDownloadWithGrantee(t *testing.T) {
	user1Wallet := "0x7d273271690538cf855e5b3002a0dd8c154bb060"
	user2Wallet := "0xc3d1d41e6872ffbd15c473d14fc3a9250be5b5e0"
	user3Wallet := "0x4954d18926ba0ed9378938444731be4e622537b2"
	app := emptyTestApp(t)

	fixtures := database.FixtureMap{
		"users": []map[string]any{
			{"user_id": 1, "handle": "seller", "name": "seller", "wallet": user1Wallet, "spl_usdc_payout_wallet": user1Wallet},
			{"user_id": 2, "handle": "buyer1", "name": "buyer1", "wallet": user2Wallet},
			{"user_id": 3, "handle": "grantee", "name": "grantee", "wallet": user3Wallet},
		},
		"tracks": []map[string]any{
			{"track_id": 1, "title": "track1", "owner_id": 1},
		},
		"track_routes": []map[string]any{
			{
				"track_id":     1,
				"owner_id":     1,
				"slug":         "track1",
				"title_slug":   "track1",
				"collision_id": 0,
			},
		},
		"sol_purchases": []map[string]any{
			{
				"signature":         "def",
				"instruction_index": 0,
				"buyer_user_id":     2,
				"country":           "US",
				"content_type":      "track",
				"content_id":        1,
				"amount":            1000000,
				"created_at":        time.Date(2024, 6, 4, 0, 0, 0, 0, time.UTC),
				"is_valid":          true,
			},
		},
		"sol_payments": []map[string]any{
			{"signature": "def", "instruction_index": 0, "route_index": 0, "to_account": user1Wallet, "amount": 900000, "slot": 101},
			{"signature": "def", "instruction_index": 0, "route_index": 1, "to_account": app.solanaConfig.StakingBridgeUsdcTokenAccount.String(), "amount": 100000, "slot": 101},
		},
		"track_price_history": []map[string]any{
			{"track_id": 1, "total_price_cents": 50, "block_timestamp": time.Date(2024, 6, 3, 0, 0, 0, 0, time.UTC), "splits": "[]"},
		},
		"grants": []map[string]any{
			{
				"user_id":         2,           // buyer1
				"grantee_address": user3Wallet, // grantee wallet
				"is_current":      true,
				"is_approved":     true,
				"is_revoked":      false,
				"created_at":      time.Now(),
				"updated_at":      time.Now(),
			},
		},
	}

	database.Seed(app.writePool, fixtures)

	// Test JSON with grantee wallet
	{
		status, body := testGetWithWallet(t, app, "/v1/users/ML51L/purchases/download/json", user3Wallet)
		assert.Equal(t, 200, status)

		jsonAssert(t, body, map[string]any{"data.#": 1})
		jsonAssert(t, body, map[string]any{"data.0.title": "track1"})
		jsonAssert(t, body, map[string]any{"data.0.link": "http://localhost:1323/seller/track1"})
		jsonAssert(t, body, map[string]any{"data.0.seller_name": "seller"})
		jsonAssert(t, body, map[string]any{"data.0.seller_user_id": 1})
		jsonAssert(t, body, map[string]any{"data.0.date": "2024-06-04T00:00:00Z"})
		jsonAssert(t, body, map[string]any{"data.0.sale_price": "1"})
		jsonAssert(t, body, map[string]any{"data.0.paid_to_artist": "0.9"})
		jsonAssert(t, body, map[string]any{"data.0.network_fee": "0.1"})
		jsonAssert(t, body, map[string]any{"data.0.pay_extra": "0.5"})
		jsonAssert(t, body, map[string]any{"data.0.total": "1.5"})
	}

	// Test CSV with grantee wallet
	{
		status, body := testGetWithWallet(t, app, "/v1/users/ML51L/purchases/download", user3Wallet)
		assert.Equal(t, 200, status)

		headers, dataRows, err := parseCSVLines(string(body))
		assert.NoError(t, err)

		// Verify headers
		expectedHeaders := []string{
			"title",
			"link",
			"artist",
			"date",
			"paid to artist",
			"network fee",
			"pay extra",
			"total",
		}
		assert.Equal(t, expectedHeaders, headers)

		assert.Equal(t, 1, len(dataRows))
		row := dataRows[0]
		assert.Equal(t, "track1", row[0])                              // title
		assert.Equal(t, "http://localhost:1323/seller/track1", row[1]) // link
		assert.Equal(t, "seller", row[2])                              // artist
		assert.Equal(t, "2024-06-04T00:00:00Z", row[3])                // date
		assert.Equal(t, "0.900000", row[4])                            // paid to artist
		assert.Equal(t, "0.100000", row[5])                            // network fee
		assert.Equal(t, "0.500000", row[6])                            // pay extra
		assert.Equal(t, "1.500000", row[7])                            // total
	}

	// Test 403 with no wallet
	{
		status, _ := testGet(t, app, "/v1/users/ML51L/purchases/download/json")
		assert.Equal(t, 403, status)
	}

	// Test 403 with unauthorized wallet (wallet without grants)
	{
		unauthorizedWallet := "0x855d28d495ec1b06364bb7a521212753e2190b95" // wallet without grants
		status, _ := testGetWithWallet(t, app, "/v1/users/ML51L/purchases/download/json", unauthorizedWallet)
		assert.Equal(t, 403, status)
	}
}
