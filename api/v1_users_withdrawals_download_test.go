package api

import (
	"testing"
	"time"

	"api.audius.co/database"
	"github.com/stretchr/testify/assert"
)

func TestV1UsersWithdrawalsDownload(t *testing.T) {
	userWallet := "0x7d273271690538cf855e5b3002a0dd8c154bb060"
	app := emptyTestApp(t)

	fixtures := database.FixtureMap{
		"users": []map[string]any{
			{"user_id": 1, "handle": "user1", "name": "user1", "wallet": userWallet, "is_current": true},
		},
		"usdc_user_bank_accounts": []map[string]any{
			{
				"bank_account":     "bank123",
				"ethereum_address": userWallet,
				"created_at":       time.Now(),
				"signature":        "sig123",
			},
		},
		"usdc_transactions_history": []map[string]any{
			{
				"user_bank":              "bank123",
				"slot":                   101,
				"signature":              "tx_sig_1",
				"transaction_type":       "transfer",
				"method":                 "send",
				"created_at":             time.Now(),
				"updated_at":             time.Now(),
				"transaction_created_at": time.Date(2024, 6, 4, 0, 0, 0, 0, time.UTC),
				"tx_metadata":            "0x1234567890abcdef1234567890abcdef12345678",
				"change":                 -500000,
				"balance":                1000000,
			},
			{
				"user_bank":              "bank123",
				"slot":                   102,
				"signature":              "tx_sig_2",
				"transaction_type":       "transfer",
				"method":                 "send",
				"created_at":             time.Now(),
				"updated_at":             time.Now(),
				"transaction_created_at": time.Date(2024, 6, 5, 0, 0, 0, 0, time.UTC),
				"tx_metadata":            "0xabcdef1234567890abcdef1234567890abcdef12",
				"change":                 -1000000,
				"balance":                500000,
			},
		},
	}

	database.Seed(app.writePool, fixtures)

	// json
	{
		status, body := testGetWithWallet(t, app, "/v1/users/7eP5n/withdrawals/download/json", userWallet)
		assert.Equal(t, 200, status)

		jsonAssert(t, body, map[string]any{"data.#": 2})
		jsonAssert(t, body, map[string]any{"data.0.destination_wallet": "0xabcdef1234567890abcdef1234567890abcdef12"})
		jsonAssert(t, body, map[string]any{"data.0.date": "2024-06-05T00:00:00Z"})
		jsonAssert(t, body, map[string]any{"data.0.amount": "1"})
		jsonAssert(t, body, map[string]any{"data.1.destination_wallet": "0x1234567890abcdef1234567890abcdef12345678"})
		jsonAssert(t, body, map[string]any{"data.1.date": "2024-06-04T00:00:00Z"})
		jsonAssert(t, body, map[string]any{"data.1.amount": "0.5"})
	}

	// csv
	{
		status, body := testGetWithWallet(t, app, "/v1/users/7eP5n/withdrawals/download", userWallet)
		assert.Equal(t, 200, status)

		headers, dataRows, err := parseCSVLines(string(body))
		assert.NoError(t, err)

		expectedHeaders := []string{
			"destination wallet",
			"date",
			"amount",
		}
		assert.Equal(t, expectedHeaders, headers)

		assert.Equal(t, 2, len(dataRows))

		row1 := dataRows[0]
		assert.Equal(t, "0xabcdef1234567890abcdef1234567890abcdef12", row1[0]) // destination wallet
		assert.Equal(t, "2024-06-05T00:00:00Z", row1[1])                       // date
		assert.Equal(t, "1.000000", row1[2])                                   // amount

		row2 := dataRows[1]
		assert.Equal(t, "0x1234567890abcdef1234567890abcdef12345678", row2[0]) // destination wallet
		assert.Equal(t, "2024-06-04T00:00:00Z", row2[1])                       // date
		assert.Equal(t, "0.500000", row2[2])                                   // amount
	}

	// Test 403 with no wallet
	{
		status, _ := testGet(t, app, "/v1/users/7eP5n/withdrawals/download/json")
		assert.Equal(t, 403, status)
	}

	// Test 403 with unauthorized wallet
	{
		unauthorizedWallet := "0x855d28d495ec1b06364bb7a521212753e2190b95" // wallet without grants
		status, _ := testGetWithWallet(t, app, "/v1/users/7eP5n/withdrawals/download/json", unauthorizedWallet)
		assert.Equal(t, 403, status)
	}
}

func TestV1UsersWithdrawalsDownloadWithGrantee(t *testing.T) {
	user1Wallet := "0x7d273271690538cf855e5b3002a0dd8c154bb060"
	user3Wallet := "0x4954d18926ba0ed9378938444731be4e622537b2"
	app := emptyTestApp(t)

	fixtures := database.FixtureMap{
		"users": []map[string]any{
			{"user_id": 1, "handle": "user1", "name": "user1", "wallet": user1Wallet, "is_current": true},
			{"user_id": 2, "handle": "grantee", "name": "grantee", "wallet": user3Wallet},
		},
		"usdc_user_bank_accounts": []map[string]any{
			{
				"bank_account":     "bank123",
				"ethereum_address": user1Wallet,
				"created_at":       time.Now(),
				"signature":        "sig123",
			},
		},
		"usdc_transactions_history": []map[string]any{
			{
				"user_bank":              "bank123",
				"slot":                   101,
				"signature":              "tx_sig_1",
				"transaction_type":       "transfer",
				"method":                 "send",
				"created_at":             time.Now(),
				"updated_at":             time.Now(),
				"transaction_created_at": time.Date(2024, 6, 4, 0, 0, 0, 0, time.UTC),
				"tx_metadata":            "0x1234567890abcdef1234567890abcdef12345678",
				"change":                 -500000,
				"balance":                1000000,
			},
			{
				"user_bank":              "bank123",
				"slot":                   102,
				"signature":              "tx_sig_2",
				"transaction_type":       "transfer",
				"method":                 "send",
				"created_at":             time.Now(),
				"updated_at":             time.Now(),
				"transaction_created_at": time.Date(2024, 6, 5, 0, 0, 0, 0, time.UTC),
				"tx_metadata":            "0xabcdef1234567890abcdef1234567890abcdef12",
				"change":                 -1000000,
				"balance":                500000,
			},
		},
		"grants": []map[string]any{
			{
				"user_id":         1,           // user1
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
		status, body := testGetWithWallet(t, app, "/v1/users/7eP5n/withdrawals/download/json", user3Wallet)
		assert.Equal(t, 200, status)

		jsonAssert(t, body, map[string]any{"data.#": 2})
		jsonAssert(t, body, map[string]any{"data.0.destination_wallet": "0xabcdef1234567890abcdef1234567890abcdef12"})
		jsonAssert(t, body, map[string]any{"data.0.date": "2024-06-05T00:00:00Z"})
		jsonAssert(t, body, map[string]any{"data.0.amount": "1"})
		jsonAssert(t, body, map[string]any{"data.1.destination_wallet": "0x1234567890abcdef1234567890abcdef12345678"})
		jsonAssert(t, body, map[string]any{"data.1.date": "2024-06-04T00:00:00Z"})
		jsonAssert(t, body, map[string]any{"data.1.amount": "0.5"})
	}

	// Test CSV with grantee wallet
	{
		status, body := testGetWithWallet(t, app, "/v1/users/7eP5n/withdrawals/download", user3Wallet)
		assert.Equal(t, 200, status)

		headers, dataRows, err := parseCSVLines(string(body))
		assert.NoError(t, err)

		expectedHeaders := []string{
			"destination wallet",
			"date",
			"amount",
		}
		assert.Equal(t, expectedHeaders, headers)

		assert.Equal(t, 2, len(dataRows))

		row1 := dataRows[0]
		assert.Equal(t, "0xabcdef1234567890abcdef1234567890abcdef12", row1[0]) // destination wallet
		assert.Equal(t, "2024-06-05T00:00:00Z", row1[1])                       // date
		assert.Equal(t, "1.000000", row1[2])                                   // amount

		row2 := dataRows[1]
		assert.Equal(t, "0x1234567890abcdef1234567890abcdef12345678", row2[0]) // destination wallet
		assert.Equal(t, "2024-06-04T00:00:00Z", row2[1])                       // date
		assert.Equal(t, "0.500000", row2[2])                                   // amount
	}

	// Test 403 with no wallet
	{
		status, _ := testGet(t, app, "/v1/users/7eP5n/withdrawals/download/json")
		assert.Equal(t, 403, status)
	}

	// Test 403 with unauthorized wallet (wallet without grants)
	{
		unauthorizedWallet := "0x855d28d495ec1b06364bb7a521212753e2190b95" // wallet without grants
		status, _ := testGetWithWallet(t, app, "/v1/users/7eP5n/withdrawals/download/json", unauthorizedWallet)
		assert.Equal(t, 403, status)
	}
}
