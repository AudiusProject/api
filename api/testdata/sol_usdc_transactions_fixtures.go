package testdata

import "time"

// USDC mint constant — duplicated here to avoid importing the api package.
const usdcMintTestData = "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v"

// User-bank addresses used by the USDC-transactions tests. Distinct from the
// AUDIO bank for the same user so the (account, mint) join in
// v_token_transactions_history disambiguates cleanly.
const (
	user1UsdcBank        = "User1UsdcBank________________________________"
	user1WithdrawDest1   = "0x1234567890abcdef1234567890abcdef12345678"
	user1WithdrawDest2   = "0xabcdef1234567890abcdef1234567890abcdef12"
	user1UsdcExternalIn  = "ExternalUsdcSender___________________________"
)

// Mirrors the legacy UsdcTransactionsHistoryFixtures rows in sol_* shape, used
// by /v1/users/{id}/transactions/usdc and /v1/users/{id}/withdrawals/download.
//
// Per legacy fixture for user 7eP5n's USDC user_bank (4 derivable rows;
// prepare_withdrawal / recover_withdrawal are system-level events with no
// underlying balance change, so they are intentionally omitted):
//   0x12345 — TRANSFER receive,        change=+100, balance=100
//   0x23456 — PURCHASE_CONTENT send,   change=-10,  balance=90
//   0x34567 — TRANSFER send,           change=-10,  balance=80
//   0x67890 — TRANSFER send (legacy: WITHDRAWAL), change=-10, balance=70
//
// The two "send transfer" rows exercise the withdrawals-download path
// (cat.to_account surfaces the destination).

var SolClaimableAccountsUsdcFixtures = []map[string]any{
	{
		"signature":         "claim_create_user1_usdc",
		"instruction_index": 0,
		"slot":              1,
		"mint":              usdcMintTestData,
		"ethereum_address":  "0x7d273271690538cf855e5b3002a0dd8c154bb060", // user 1
		"account":           user1UsdcBank,
	},
}

var SolTokenAccountBalanceChangesUsdcFixtures = []map[string]any{
	{
		"signature":       "0x12345_usdc",
		"mint":            usdcMintTestData,
		"owner":           "claimable-tokens-pda",
		"account":         user1UsdcBank,
		"change":          100,
		"balance":         100,
		"slot":            10,
		"block_timestamp": time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC),
	},
	{
		"signature":       "0x23456_usdc",
		"mint":            usdcMintTestData,
		"owner":           "claimable-tokens-pda",
		"account":         user1UsdcBank,
		"change":          -10,
		"balance":         90,
		"slot":            20,
		"block_timestamp": time.Date(2021, 1, 2, 0, 0, 0, 0, time.UTC),
	},
	{
		"signature":       "0x34567_usdc",
		"mint":            usdcMintTestData,
		"owner":           "claimable-tokens-pda",
		"account":         user1UsdcBank,
		"change":          -500000,
		"balance":         80,
		"slot":            30,
		"block_timestamp": time.Date(2024, 6, 4, 0, 0, 0, 0, time.UTC),
	},
	{
		"signature":       "0x67890_usdc",
		"mint":            usdcMintTestData,
		"owner":           "claimable-tokens-pda",
		"account":         user1UsdcBank,
		"change":          -1000000,
		"balance":         70,
		"slot":            40,
		"block_timestamp": time.Date(2024, 6, 5, 0, 0, 0, 0, time.UTC),
	},
}

var SolClaimableAccountTransfersUsdcFixtures = []map[string]any{
	// TRANSFER receive from external sender.
	{
		"signature":          "0x12345_usdc",
		"instruction_index":  0,
		"amount":             100,
		"slot":               10,
		"from_account":       user1UsdcExternalIn,
		"to_account":         user1UsdcBank,
		"sender_eth_address": "0xdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef",
	},
	// TRANSFER send to external (legacy fixture 0x34567 — generic transfer).
	{
		"signature":          "0x34567_usdc",
		"instruction_index":  0,
		"amount":             500000,
		"slot":               30,
		"from_account":       user1UsdcBank,
		"to_account":         user1WithdrawDest1,
		"sender_eth_address": "0x7d273271690538cf855e5b3002a0dd8c154bb060",
	},
	// TRANSFER send to external (legacy fixture 0x67890 — was "withdrawal";
	// now indistinguishable from any other send-to-external).
	{
		"signature":          "0x67890_usdc",
		"instruction_index":  0,
		"amount":             1000000,
		"slot":               40,
		"from_account":       user1UsdcBank,
		"to_account":         user1WithdrawDest2,
		"sender_eth_address": "0x7d273271690538cf855e5b3002a0dd8c154bb060",
	},
}

// PURCHASE_CONTENT — matches the legacy 0x23456 send purchase_content row.
// Added to SolPurchasesFixtures so v_token_transactions_history classifies it
// correctly.
var SolPurchasesUsdcFixtures = []map[string]any{
	{
		"signature":         "0x23456_usdc",
		"instruction_index": 0,
		"slot":              20,
		"from_account":      user1UsdcBank,
		"buyer_user_id":     1,
		"content_id":        303,
		"content_type":      "track",
		"amount":            10,
		"is_valid":          true,
	},
}
