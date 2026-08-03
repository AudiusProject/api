package api

import (
	"testing"
	"time"

	"api.audius.co/database"
	"api.audius.co/trashid"
	"github.com/stretchr/testify/assert"
)

func TestV1UsersSalesDownload(t *testing.T) {
	user1Wallet := "0x7d273271690538cf855e5b3002a0dd8c154bb060"
	app := emptyTestApp(t)

	fixtures := database.FixtureMap{
		"users": []map[string]any{
			{"user_id": 1, "handle": "seller", "wallet": user1Wallet, "spl_usdc_payout_wallet": user1Wallet},
			{"user_id": 2, "handle": "buyer1", "name": "buyer1"},
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
		"user_payout_wallet_history": []map[string]any{
			{"user_id": 1, "spl_usdc_payout_wallet": user1Wallet, "block_timestamp": time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC)},
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
		"encrypted_emails": []map[string]any{
			{
				"id":                  1,
				"email_owner_user_id": 2, // buyer1
				"encrypted_email":     "encrypted_email_data_123",
				"created_at":          time.Now(),
				"updated_at":          time.Now(),
			},
		},
		"email_access": []map[string]any{
			{
				"id":                  1,
				"email_owner_user_id": 2, // buyer1
				"receiving_user_id":   1, // seller
				"grantor_user_id":     2, // buyer1
				"encrypted_key":       "encrypted_key_data_456",
				"is_initial":          true,
				"created_at":          time.Now(),
				"updated_at":          time.Now(),
			},
		},
		"user_pubkeys": []map[string]any{
			{
				"user_id":       2, // buyer1
				"pubkey_base64": "pubkey_base64_data_789",
			},
		},
	}

	database.Seed(app.writePool, fixtures)

	{
		status, body := testGetWithWallet(t, app, "/v1/users/7eP5n/sales/download/json", user1Wallet)
		assert.Equal(t, 200, status)

		jsonAssert(t, body, map[string]any{"data.sales.#": 1})
		jsonAssert(t, body, map[string]any{"data.sales.0.title": "track1"})
		jsonAssert(t, body, map[string]any{"data.sales.0.link": "http://localhost:1323/seller/track1"})
		jsonAssert(t, body, map[string]any{"data.sales.0.purchased_by": "buyer1"})
		jsonAssert(t, body, map[string]any{"data.sales.0.date": "2024-06-04T00:00:00Z"})
		jsonAssert(t, body, map[string]any{"data.sales.0.sale_price": "1"})
		jsonAssert(t, body, map[string]any{"data.sales.0.network_fee": "-0.1"})
		jsonAssert(t, body, map[string]any{"data.sales.0.pay_extra": "0.5"})
		jsonAssert(t, body, map[string]any{"data.sales.0.total": "1.4"})
		jsonAssert(t, body, map[string]any{"data.sales.0.country": "US"})
		jsonAssert(t, body, map[string]any{"data.sales.0.encrypted_email": "encrypted_email_data_123"})
		jsonAssert(t, body, map[string]any{"data.sales.0.encrypted_key": "encrypted_key_data_456"})
		jsonAssert(t, body, map[string]any{"data.sales.0.is_initial": true})
		jsonAssert(t, body, map[string]any{"data.sales.0.pubkey_base64": "pubkey_base64_data_789"})
	}

	// csv
	{
		status, body := testGetWithWallet(t, app, "/v1/users/7eP5n/sales/download", user1Wallet)
		assert.Equal(t, 200, status)

		headers, dataRows, err := parseCSVLines(string(body))
		assert.NoError(t, err)

		// Verify headers
		expectedHeaders := []string{
			"title",
			"link",
			"purchased by",
			"date",
			"sale price",
			"network fee",
			"pay extra",
			"total",
			"country",
		}
		assert.Equal(t, expectedHeaders, headers)

		assert.Equal(t, 1, len(dataRows))
		row := dataRows[0]
		assert.Equal(t, "track1", row[0])                              // title
		assert.Equal(t, "http://localhost:1323/seller/track1", row[1]) // link
		assert.Equal(t, "buyer1", row[2])                              // purchased by
		assert.Equal(t, "2024-06-04T00:00:00Z", row[3])                // date
		assert.Equal(t, "1.000000", row[4])                            // sale price
		assert.Equal(t, "-0.100000", row[5])                           // network fee
		assert.Equal(t, "0.500000", row[6])                            // pay extra
		assert.Equal(t, "1.400000", row[7])                            // total
		assert.Equal(t, "US", row[8])                                  // country
	}

	// Test 403 with no wallet
	{
		status, _ := testGet(t, app, "/v1/users/7eP5n/sales/download/json")
		assert.Equal(t, 403, status)
	}

	// Test 403 with unauthorized wallet
	{
		unauthorizedWallet := "0x855d28d495ec1b06364bb7a521212753e2190b95" // wallet without grants
		status, _ := testGetWithWallet(t, app, "/v1/users/7eP5n/sales/download/json", unauthorizedWallet)
		assert.Equal(t, 403, status)
	}
}

func TestV1UsersSalesDownloadWithGrantee(t *testing.T) {
	user1Wallet := "0x7d273271690538cf855e5b3002a0dd8c154bb060"
	user2Wallet := "0xc3d1d41e6872ffbd15c473d14fc3a9250be5b5e0"
	user3Wallet := "0x4954d18926ba0ed9378938444731be4e622537b2"
	app := emptyTestApp(t)

	fixtures := database.FixtureMap{
		"users": []map[string]any{
			{"user_id": 1, "handle": "seller", "wallet": user1Wallet, "spl_usdc_payout_wallet": user1Wallet},
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
		"user_payout_wallet_history": []map[string]any{
			{"user_id": 1, "spl_usdc_payout_wallet": user1Wallet, "block_timestamp": time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC)},
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
		"encrypted_emails": []map[string]any{
			{
				"id":                  1,
				"email_owner_user_id": 2, // buyer1
				"encrypted_email":     "encrypted_email_data_123",
				"created_at":          time.Now(),
				"updated_at":          time.Now(),
			},
		},
		"email_access": []map[string]any{
			{
				"id":                  1,
				"email_owner_user_id": 2, // buyer1
				"receiving_user_id":   3, // grantee (not seller)
				"grantor_user_id":     1, // seller
				"encrypted_key":       "encrypted_key_data_456",
				"is_initial":          true,
				"created_at":          time.Now(),
				"updated_at":          time.Now(),
			},
			{
				"id":                  2,
				"email_owner_user_id": 2, // buyer1
				"receiving_user_id":   1, // seller (for direct access)
				"grantor_user_id":     2, // buyer1
				"encrypted_key":       "encrypted_key_data_789",
				"is_initial":          false,
				"created_at":          time.Now(),
				"updated_at":          time.Now(),
			},
		},
		"user_pubkeys": []map[string]any{
			{
				"user_id":       2, // buyer1
				"pubkey_base64": "pubkey_base64_data_789",
			},
		},
		"grants": []map[string]any{
			{
				"user_id":         1,           // seller
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

	// Test with grantee_user_id parameter
	{
		granteeHashId := trashid.MustEncodeHashID(3)
		status, body := testGetWithWallet(t, app, "/v1/users/7eP5n/sales/download/json?grantee_user_id="+granteeHashId, user3Wallet)
		assert.Equal(t, 200, status)

		jsonAssert(t, body, map[string]any{"data.sales.#": 1})
		jsonAssert(t, body, map[string]any{"data.sales.0.title": "track1"})
		jsonAssert(t, body, map[string]any{"data.sales.0.link": "http://localhost:1323/seller/track1"})
		jsonAssert(t, body, map[string]any{"data.sales.0.purchased_by": "buyer1"})
		jsonAssert(t, body, map[string]any{"data.sales.0.date": "2024-06-04T00:00:00Z"})
		jsonAssert(t, body, map[string]any{"data.sales.0.sale_price": "1"})
		jsonAssert(t, body, map[string]any{"data.sales.0.network_fee": "-0.1"})
		jsonAssert(t, body, map[string]any{"data.sales.0.pay_extra": "0.5"})
		jsonAssert(t, body, map[string]any{"data.sales.0.total": "1.4"})
		jsonAssert(t, body, map[string]any{"data.sales.0.country": "US"})
		// Email access should be available to the grantee
		jsonAssert(t, body, map[string]any{"data.sales.0.encrypted_email": "encrypted_email_data_123"})
		jsonAssert(t, body, map[string]any{"data.sales.0.encrypted_key": "encrypted_key_data_456"})
		jsonAssert(t, body, map[string]any{"data.sales.0.is_initial": true})
		jsonAssert(t, body, map[string]any{"data.sales.0.pubkey_base64": "pubkey_base64_data_789"})
	}

	// csv
	{
		granteeHashId := trashid.MustEncodeHashID(3)
		status, body := testGetWithWallet(t, app, "/v1/users/7eP5n/sales/download?grantee_user_id="+granteeHashId, user3Wallet)
		assert.Equal(t, 200, status)

		headers, dataRows, err := parseCSVLines(string(body))
		assert.NoError(t, err)

		// Verify headers
		expectedHeaders := []string{
			"title",
			"link",
			"purchased by",
			"date",
			"sale price",
			"network fee",
			"pay extra",
			"total",
			"country",
		}
		assert.Equal(t, expectedHeaders, headers)

		assert.Equal(t, 1, len(dataRows))
		row := dataRows[0]
		assert.Equal(t, "track1", row[0])                              // title
		assert.Equal(t, "http://localhost:1323/seller/track1", row[1]) // link
		assert.Equal(t, "buyer1", row[2])                              // purchased by
		assert.Equal(t, "2024-06-04T00:00:00Z", row[3])                // date
		assert.Equal(t, "1.000000", row[4])                            // sale price
		assert.Equal(t, "-0.100000", row[5])                           // network_fee
		assert.Equal(t, "0.500000", row[6])                            // pay extra
		assert.Equal(t, "1.400000", row[7])                            // total
		assert.Equal(t, "US", row[8])                                  // country
	}

	// Test 403 with no wallet
	{
		status, _ := testGet(t, app, "/v1/users/7eP5n/sales/download/json")
		assert.Equal(t, 403, status)
	}

	// Test 403 with unauthorized wallet (wallet without grants)
	{
		unauthorizedWallet := "0x855d28d495ec1b06364bb7a521212753e2190b95" // wallet without grants
		status, _ := testGetWithWallet(t, app, "/v1/users/7eP5n/sales/download/json", unauthorizedWallet)
		assert.Equal(t, 403, status)
	}

}
