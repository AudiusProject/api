package testdata

import "time"

// USDC mint constant — duplicated here to avoid importing the api package.
const usdcMintTestData = "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v"

// User-bank addresses used by the USDC-transactions tests. Distinct from the
// AUDIO bank for the same user so the (account, mint) join in
// v_token_transactions_history disambiguates cleanly.
const (
	user1UsdcBank        = "User1UsdcBank________________________________"
	user2UsdcBank        = "User2UsdcBank________________________________"
	user1WithdrawDest1   = "0x1234567890abcdef1234567890abcdef12345678"
	user1WithdrawDest2   = "0xabcdef1234567890abcdef1234567890abcdef12"
	user1UsdcExternalIn  = "ExternalUsdcSender___________________________"
	jupiterRoutedToAcct  = "JupiterDestinationTokenAcct_________________"
	recoverDestinationCT = "RecoverWithdrawalDestination________________"
)

// Fixtures for the USDC user_bank covering every memo-derived transaction_type
// that v_token_transactions_history can return after #858:
//
//   0x12345_usdc — TRANSFER receive (from external)
//   0x23456_usdc — PURCHASE_CONTENT (sol_purchases row)
//   0x34567_usdc — TRANSFER send (no memo)
//   0x67890_usdc — WITHDRAWAL (memo marker)
//   0x78901_usdc — PREPARE_WITHDRAWAL (memo marker)
//   0x89012_usdc — RECOVER_WITHDRAWAL (memo marker, on payment_router)
//   0x90123_usdc — INTERNAL_TRANSFER (memo marker, user1 → user2)
//
// The memo markers are populated by the program indexer at write time; here we
// seed them directly to exercise the view's CASE branches.

var SolClaimableAccountsUsdcFixtures = []map[string]any{
	{
		"signature":         "claim_create_user1_usdc",
		"instruction_index": 0,
		"slot":              1,
		"mint":              usdcMintTestData,
		"ethereum_address":  "0x7d273271690538cf855e5b3002a0dd8c154bb060", // user 1
		"account":           user1UsdcBank,
	},
	{
		"signature":         "claim_create_user2_usdc",
		"instruction_index": 0,
		"slot":              2,
		"mint":              usdcMintTestData,
		"ethereum_address":  "0x1234567890abcdef", // user 2 (matches existing fixtures)
		"account":           user2UsdcBank,
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
	{
		"signature":       "0x78901_usdc",
		"mint":            usdcMintTestData,
		"owner":           "claimable-tokens-pda",
		"account":         user1UsdcBank,
		"change":          -2000000,
		"balance":         50,
		"slot":            50,
		"block_timestamp": time.Date(2024, 6, 6, 0, 0, 0, 0, time.UTC),
	},
	{
		"signature":       "0x89012_usdc",
		"mint":            usdcMintTestData,
		"owner":           "claimable-tokens-pda",
		"account":         user1UsdcBank,
		"change":          1500000,
		"balance":         1550,
		"slot":            60,
		"block_timestamp": time.Date(2024, 6, 7, 0, 0, 0, 0, time.UTC),
	},
	{
		"signature":       "0x90123_usdc",
		"mint":            usdcMintTestData,
		"owner":           "claimable-tokens-pda",
		"account":         user1UsdcBank,
		"change":          -300000,
		"balance":         50,
		"slot":            70,
		"block_timestamp": time.Date(2024, 6, 8, 0, 0, 0, 0, time.UTC),
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
	// TRANSFER send to external — no memo marker, plain transfer.
	{
		"signature":          "0x34567_usdc",
		"instruction_index":  0,
		"amount":             500000,
		"slot":               30,
		"from_account":       user1UsdcBank,
		"to_account":         user1WithdrawDest1,
		"sender_eth_address": "0x7d273271690538cf855e5b3002a0dd8c154bb060",
	},
	// WITHDRAWAL — memo-tagged in sol_transfer_memo_types below.
	{
		"signature":          "0x67890_usdc",
		"instruction_index":  0,
		"amount":             1000000,
		"slot":               40,
		"from_account":       user1UsdcBank,
		"to_account":         user1WithdrawDest2,
		"sender_eth_address": "0x7d273271690538cf855e5b3002a0dd8c154bb060",
	},
	// PREPARE_WITHDRAWAL — claimable transfer to a Jupiter-routed token account.
	{
		"signature":          "0x78901_usdc",
		"instruction_index":  0,
		"amount":             2000000,
		"slot":               50,
		"from_account":       user1UsdcBank,
		"to_account":         jupiterRoutedToAcct,
		"sender_eth_address": "0x7d273271690538cf855e5b3002a0dd8c154bb060",
	},
	// INTERNAL_TRANSFER — user1 → user2 (both known user_banks).
	{
		"signature":          "0x90123_usdc",
		"instruction_index":  0,
		"amount":             300000,
		"slot":               70,
		"from_account":       user1UsdcBank,
		"to_account":         user2UsdcBank,
		"sender_eth_address": "0x7d273271690538cf855e5b3002a0dd8c154bb060",
	},
}

// RECOVER_WITHDRAWAL is a payment_router route (not a claimable transfer), so
// it lives in sol_payments instead of sol_claimable_account_transfers.
var SolPaymentsUsdcFixtures = []map[string]any{
	{
		"signature":         "0x89012_usdc",
		"instruction_index": 0,
		"route_index":       0,
		"to_account":        user1UsdcBank,
		"amount":            1500000,
		"slot":              60,
	},
}

// Memo markers — populated by the program/token indexer when it spots one of
// the recognized memo strings. Seeded here so v_token_transactions_history's
// CASE picks the right transaction_type.
var SolTransferMemoTypesUsdcFixtures = []map[string]any{
	{"signature": "0x67890_usdc", "instruction_index": 0, "slot": 40, "memo_type": "withdrawal"},
	{"signature": "0x78901_usdc", "instruction_index": 0, "slot": 50, "memo_type": "prepare_withdrawal"},
	{"signature": "0x89012_usdc", "instruction_index": 0, "slot": 60, "memo_type": "recover_withdrawal"},
	{"signature": "0x90123_usdc", "instruction_index": 0, "slot": 70, "memo_type": "internal_transfer"},
}

// PURCHASE_CONTENT — matches the 0x23456 send purchase_content row.
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
