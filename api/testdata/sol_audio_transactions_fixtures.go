package testdata

import "time"

// wAUDIO mint constant — duplicated here to avoid importing the api package.
const wAudioMintTestData = "9LzCMqDgTKYz9Drzqnpgee3SGa89up3a247ypMj2xrqM"

// Bank accounts used by the audio-transactions tests.
const (
	user1AudioBank        = "DsUGy77ssRh9EXzef3AZLLT9GQBuyqHRdhkBkfqQ3x1D"      // user 7eP5n (user_id=1)
	user2AudioBank        = "User2AudioBank_______________________________"     // user_id=2 (tip counterpart)
	externalAudioAccount  = "ExternalNonUserBank__________________________"     // not in sol_claimable_accounts (transfer counterpart)
)

// Mirrors the rows in AudioTransactionsHistory but in sol_* shape, used by
// /v1/users/{id}/transactions/audio and the v_token_transactions_history view.
//
// Per legacy fixture for user 7eP5n's user_bank (5 rows, distinct dates):
//   0x12345 — TIP receive,   change=+100, balance=100
//   0x23456 — TIP send,      change=-10,  balance=90
//   0x34567 — TIP send,      change=-10,  balance=80
//   0x45678 — TRANSFER send, change=-50,  balance=30
//   0x56789 — TRANSFER send, change=-10,  balance=20
//
// Tips resolve when both endpoints map to known user_banks of distinct users.
// Transfers are claimable transfers where the counterpart is NOT a known
// user_bank (here: externalAudioAccount).

var SolClaimableAccountsAudioFixtures = []map[string]any{
	{
		"signature":         "claim_create_user1",
		"instruction_index": 0,
		"slot":              1,
		"mint":              wAudioMintTestData,
		"ethereum_address":  "0x7d273271690538cf855e5b3002a0dd8c154bb060", // user 1
		"account":           user1AudioBank,
	},
	{
		"signature":         "claim_create_user2",
		"instruction_index": 0,
		"slot":              1,
		"mint":              wAudioMintTestData,
		"ethereum_address":  "0x1234567890abcdef", // user 2 (stereosteve)
		"account":           user2AudioBank,
	},
}

var SolTokenAccountBalanceChangesAudioFixtures = []map[string]any{
	{
		"signature":       "0x12345",
		"mint":            wAudioMintTestData,
		"owner":           "claimable-tokens-pda",
		"account":         user1AudioBank,
		"change":          100,
		"balance":         100,
		"slot":            10,
		"block_timestamp": time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC),
	},
	{
		"signature":       "0x23456",
		"mint":            wAudioMintTestData,
		"owner":           "claimable-tokens-pda",
		"account":         user1AudioBank,
		"change":          -10,
		"balance":         90,
		"slot":            20,
		"block_timestamp": time.Date(2021, 1, 2, 0, 0, 0, 0, time.UTC),
	},
	{
		"signature":       "0x34567",
		"mint":            wAudioMintTestData,
		"owner":           "claimable-tokens-pda",
		"account":         user1AudioBank,
		"change":          -10,
		"balance":         80,
		"slot":            30,
		"block_timestamp": time.Date(2021, 1, 3, 0, 0, 0, 0, time.UTC),
	},
	{
		"signature":       "0x45678",
		"mint":            wAudioMintTestData,
		"owner":           "claimable-tokens-pda",
		"account":         user1AudioBank,
		"change":          -50,
		"balance":         30,
		"slot":            40,
		"block_timestamp": time.Date(2021, 1, 4, 0, 0, 0, 0, time.UTC),
	},
	{
		"signature":       "0x56789",
		"mint":            wAudioMintTestData,
		"owner":           "claimable-tokens-pda",
		"account":         user1AudioBank,
		"change":          -10,
		"balance":         20,
		"slot":            50,
		"block_timestamp": time.Date(2021, 1, 5, 0, 0, 0, 0, time.UTC),
	},
}

var SolClaimableAccountTransfersAudioFixtures = []map[string]any{
	// TIP receive: user2 -> user1 (both known user_banks)
	{
		"signature":          "0x12345",
		"instruction_index":  0,
		"amount":             100,
		"slot":               10,
		"from_account":       user2AudioBank,
		"to_account":         user1AudioBank,
		"sender_eth_address": "0x1234567890abcdef",
	},
	// TIP send: user1 -> user2
	{
		"signature":          "0x23456",
		"instruction_index":  0,
		"amount":             10,
		"slot":               20,
		"from_account":       user1AudioBank,
		"to_account":         user2AudioBank,
		"sender_eth_address": "0x7d273271690538cf855e5b3002a0dd8c154bb060",
	},
	// TIP send: user1 -> user2 (another one)
	{
		"signature":          "0x34567",
		"instruction_index":  0,
		"amount":             10,
		"slot":               30,
		"from_account":       user1AudioBank,
		"to_account":         user2AudioBank,
		"sender_eth_address": "0x7d273271690538cf855e5b3002a0dd8c154bb060",
	},
	// TRANSFER send: user1 -> external (recipient is not a known user_bank)
	{
		"signature":          "0x45678",
		"instruction_index":  0,
		"amount":             50,
		"slot":               40,
		"from_account":       user1AudioBank,
		"to_account":         externalAudioAccount,
		"sender_eth_address": "0x7d273271690538cf855e5b3002a0dd8c154bb060",
	},
	// TRANSFER send: user1 -> external (another one)
	{
		"signature":          "0x56789",
		"instruction_index":  0,
		"amount":             10,
		"slot":               50,
		"from_account":       user1AudioBank,
		"to_account":         externalAudioAccount,
		"sender_eth_address": "0x7d273271690538cf855e5b3002a0dd8c154bb060",
	},
}
