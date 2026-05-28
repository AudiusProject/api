package api

import (
	"testing"
	"time"

	"api.audius.co/database"
	"github.com/stretchr/testify/assert"
)

// withdrawalFixtures seeds the sol_* tables that
// /v1/users/{id}/withdrawals/download reads from. Three claimable transfers:
//   - tx_sig_withdraw_a / tx_sig_withdraw_b are tagged `withdrawal` via memo
//     markers and surface in the download
//   - tx_sig_plain_xfer is a bare transfer (no memo marker) and must NOT
//     surface, since the endpoint is now scoped to actual withdrawals
//
// destination_wallet is resolved through sol_token_account_balances.owner,
// so each destination token account is seeded with a known owner wallet.
func withdrawalFixtures(userWallet string) database.FixtureMap {
	usdcBank := "User1UsdcBank_withdrawals_test_____________"
	const (
		destTokenAcctA = "DestUsdcTokenAccountA_______________________"
		destTokenAcctB = "DestUsdcTokenAccountB_______________________"
		destOwnerA     = "0x1234567890abcdef1234567890abcdef12345678"
		destOwnerB     = "0xabcdef1234567890abcdef1234567890abcdef12"
	)
	return database.FixtureMap{
		"users": []map[string]any{
			{"user_id": 1, "handle": "user1", "name": "user1", "wallet": userWallet, "is_current": true},
		},
		"sol_claimable_accounts": []map[string]any{
			{
				"signature":         "withdrawal_test_create",
				"instruction_index": 0,
				"slot":              1,
				"mint":              "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v", // USDC
				"ethereum_address":  userWallet,
				"account":           usdcBank,
			},
		},
		"sol_token_account_balance_changes": []map[string]any{
			{
				"signature":       "tx_sig_withdraw_a",
				"mint":            "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v",
				"owner":           "claimable-tokens-pda",
				"account":         usdcBank,
				"change":          -500000,
				"balance":         1000000,
				"slot":            101,
				"block_timestamp": time.Date(2024, 6, 4, 0, 0, 0, 0, time.UTC),
			},
			{
				"signature":       "tx_sig_withdraw_b",
				"mint":            "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v",
				"owner":           "claimable-tokens-pda",
				"account":         usdcBank,
				"change":          -1000000,
				"balance":         500000,
				"slot":            102,
				"block_timestamp": time.Date(2024, 6, 5, 0, 0, 0, 0, time.UTC),
			},
			{
				"signature":       "tx_sig_plain_xfer",
				"mint":            "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v",
				"owner":           "claimable-tokens-pda",
				"account":         usdcBank,
				"change":          -250000,
				"balance":         250000,
				"slot":            103,
				"block_timestamp": time.Date(2024, 6, 6, 0, 0, 0, 0, time.UTC),
			},
		},
		"sol_claimable_account_transfers": []map[string]any{
			{
				"signature":          "tx_sig_withdraw_a",
				"instruction_index":  0,
				"amount":             500000,
				"slot":               101,
				"from_account":       usdcBank,
				"to_account":         destTokenAcctA,
				"sender_eth_address": userWallet,
			},
			{
				"signature":          "tx_sig_withdraw_b",
				"instruction_index":  0,
				"amount":             1000000,
				"slot":               102,
				"from_account":       usdcBank,
				"to_account":         destTokenAcctB,
				"sender_eth_address": userWallet,
			},
			{
				"signature":          "tx_sig_plain_xfer",
				"instruction_index":  0,
				"amount":             250000,
				"slot":               103,
				"from_account":       usdcBank,
				"to_account":         "0xshouldnotappear_______________________________",
				"sender_eth_address": userWallet,
			},
		},
		// Only the two `withdraw_*` transfers are tagged — the plain transfer
		// has no marker and must be excluded from the download.
		"sol_transfer_memo_types": []map[string]any{
			{"signature": "tx_sig_withdraw_a", "instruction_index": 0, "slot": 101, "memo_type": "withdrawal"},
			{"signature": "tx_sig_withdraw_b", "instruction_index": 0, "slot": 102, "memo_type": "withdrawal"},
		},
		// Destination token accounts on Solana, with their resolved wallet
		// owners (what destination_wallet ultimately surfaces).
		"sol_token_account_balances": []map[string]any{
			{"account": destTokenAcctA, "mint": "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v", "owner": destOwnerA, "balance": 500000, "slot": 101},
			{"account": destTokenAcctB, "mint": "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v", "owner": destOwnerB, "balance": 1000000, "slot": 102},
		},
	}
}

func TestV1UsersWithdrawalsDownload(t *testing.T) {
	userWallet := "0x7d273271690538cf855e5b3002a0dd8c154bb060"
	app := emptyTestApp(t)
	database.Seed(app.writePool, withdrawalFixtures(userWallet))

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
		assert.Equal(t, "0xabcdef1234567890abcdef1234567890abcdef12", row1[0])
		assert.Equal(t, "2024-06-05T00:00:00Z", row1[1])
		assert.Equal(t, "1.000000", row1[2])

		row2 := dataRows[1]
		assert.Equal(t, "0x1234567890abcdef1234567890abcdef12345678", row2[0])
		assert.Equal(t, "2024-06-04T00:00:00Z", row2[1])
		assert.Equal(t, "0.500000", row2[2])
	}

	// Test 403 with no wallet
	{
		status, _ := testGet(t, app, "/v1/users/7eP5n/withdrawals/download/json")
		assert.Equal(t, 403, status)
	}

	// Test 403 with unauthorized wallet
	{
		unauthorizedWallet := "0x855d28d495ec1b06364bb7a521212753e2190b95"
		status, _ := testGetWithWallet(t, app, "/v1/users/7eP5n/withdrawals/download/json", unauthorizedWallet)
		assert.Equal(t, 403, status)
	}
}

func TestV1UsersWithdrawalsDownloadWithGrantee(t *testing.T) {
	user1Wallet := "0x7d273271690538cf855e5b3002a0dd8c154bb060"
	user3Wallet := "0x4954d18926ba0ed9378938444731be4e622537b2"
	app := emptyTestApp(t)

	fixtures := withdrawalFixtures(user1Wallet)
	fixtures["users"] = append(fixtures["users"],
		map[string]any{"user_id": 2, "handle": "grantee", "name": "grantee", "wallet": user3Wallet},
	)
	fixtures["grants"] = []map[string]any{
		{
			"user_id":         1,
			"grantee_address": user3Wallet,
			"is_current":      true,
			"is_approved":     true,
			"is_revoked":      false,
			"created_at":      time.Now(),
			"updated_at":      time.Now(),
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
		assert.Equal(t, "0xabcdef1234567890abcdef1234567890abcdef12", row1[0])
		assert.Equal(t, "2024-06-05T00:00:00Z", row1[1])
		assert.Equal(t, "1.000000", row1[2])

		row2 := dataRows[1]
		assert.Equal(t, "0x1234567890abcdef1234567890abcdef12345678", row2[0])
		assert.Equal(t, "2024-06-04T00:00:00Z", row2[1])
		assert.Equal(t, "0.500000", row2[2])
	}

	// Test 403 with no wallet
	{
		status, _ := testGet(t, app, "/v1/users/7eP5n/withdrawals/download/json")
		assert.Equal(t, 403, status)
	}

	// Test 403 with unauthorized wallet (wallet without grants)
	{
		unauthorizedWallet := "0x855d28d495ec1b06364bb7a521212753e2190b95"
		status, _ := testGetWithWallet(t, app, "/v1/users/7eP5n/withdrawals/download/json", unauthorizedWallet)
		assert.Equal(t, 403, status)
	}
}
