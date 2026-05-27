package api

import (
	"testing"
	"time"

	"api.audius.co/database"
	"api.audius.co/trashid"
	"github.com/stretchr/testify/assert"
)

// Tests for v_token_transactions_history transaction_type derivation rules,
// going through the /v1/users/{id}/transactions/audio endpoint. The view's
// type derivation is order-sensitive and easy to get wrong; these pin the
// behavior for each branch of the CASE statement.

const (
	tthWAudioMint    = "9LzCMqDgTKYz9Drzqnpgee3SGa89up3a247ypMj2xrqM"
	tthUserAWallet   = "0xtth_user_a"
	tthUserBWallet   = "0xtth_user_b"
	tthUserABank     = "TthUserABank____________________________________"
	tthUserBBank     = "TthUserBBank____________________________________"
	tthExternalAcct  = "TthExternalAccount______________________________"
	tthClaimablePDA  = "claimable-tokens-pda"
)

func tthBaseFixtures(extra database.FixtureMap) database.FixtureMap {
	fixtures := database.FixtureMap{
		"users": []map[string]any{
			{"user_id": 1, "handle": "userA", "wallet": tthUserAWallet},
			{"user_id": 2, "handle": "userB", "wallet": tthUserBWallet},
		},
		"sol_claimable_accounts": []map[string]any{
			{"signature": "claim_a", "instruction_index": 0, "slot": 1, "mint": tthWAudioMint, "ethereum_address": tthUserAWallet, "account": tthUserABank},
			{"signature": "claim_b", "instruction_index": 0, "slot": 1, "mint": tthWAudioMint, "ethereum_address": tthUserBWallet, "account": tthUserBBank},
		},
	}
	for table, rows := range extra {
		fixtures[table] = rows
	}
	return fixtures
}

// TIP: claimable transfer between two distinct user_banks.
func TestVTokenTransactionsHistory_Tip(t *testing.T) {
	app := emptyTestApp(t)

	purchaseTime := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)

	fixtures := tthBaseFixtures(database.FixtureMap{
		"sol_token_account_balance_changes": []map[string]any{
			// User A's side: change=-10 (send).
			{"signature": "tip_sig", "mint": tthWAudioMint, "owner": tthClaimablePDA, "account": tthUserABank, "change": -10, "balance": 90, "slot": 10, "block_timestamp": purchaseTime},
			// User B's side: change=+10 (receive).
			{"signature": "tip_sig", "mint": tthWAudioMint, "owner": tthClaimablePDA, "account": tthUserBBank, "change": 10, "balance": 110, "slot": 10, "block_timestamp": purchaseTime},
		},
		"sol_claimable_account_transfers": []map[string]any{
			{"signature": "tip_sig", "instruction_index": 0, "amount": 10, "slot": 10, "from_account": tthUserABank, "to_account": tthUserBBank, "sender_eth_address": tthUserAWallet},
		},
	})
	database.Seed(app.pool.Replicas[0], fixtures)

	// Both sides should classify as tip.
	status, body := testGet(t, app, "/v1/users/"+trashid.MustEncodeHashID(1)+"/transactions/audio")
	assert.Equal(t, 200, status)
	jsonAssert(t, body, map[string]any{
		"data.0.transaction_type": "tip",
		"data.0.method":           "send",
		"data.0.change":           float64(10),
		"data.0.metadata":         "2", // counterpart user_id
	})

	status, body = testGet(t, app, "/v1/users/"+trashid.MustEncodeHashID(2)+"/transactions/audio")
	assert.Equal(t, 200, status)
	jsonAssert(t, body, map[string]any{
		"data.0.transaction_type": "tip",
		"data.0.method":           "receive",
		"data.0.change":           float64(10),
		"data.0.metadata":         "1",
	})
}

// TRANSFER: claimable transfer where the counterpart is not a known user_bank
// (e.g. an external wallet).
func TestVTokenTransactionsHistory_Transfer(t *testing.T) {
	app := emptyTestApp(t)

	t0 := time.Date(2024, 2, 1, 0, 0, 0, 0, time.UTC)

	fixtures := tthBaseFixtures(database.FixtureMap{
		"sol_token_account_balance_changes": []map[string]any{
			{"signature": "xfer_sig", "mint": tthWAudioMint, "owner": tthClaimablePDA, "account": tthUserABank, "change": -50, "balance": 50, "slot": 20, "block_timestamp": t0},
		},
		"sol_claimable_account_transfers": []map[string]any{
			// to_account is an external account NOT in sol_claimable_accounts.
			{"signature": "xfer_sig", "instruction_index": 0, "amount": 50, "slot": 20, "from_account": tthUserABank, "to_account": tthExternalAcct, "sender_eth_address": tthUserAWallet},
		},
	})
	database.Seed(app.pool.Replicas[0], fixtures)

	status, body := testGet(t, app, "/v1/users/"+trashid.MustEncodeHashID(1)+"/transactions/audio")
	assert.Equal(t, 200, status)
	jsonAssert(t, body, map[string]any{
		"data.0.transaction_type": "transfer",
		"data.0.method":           "send",
		"data.0.change":           float64(50),
		"data.0.metadata":         nil,
	})
}

// USER_REWARD: sol_reward_disbursements row whose challenge is non-trending.
func TestVTokenTransactionsHistory_UserReward(t *testing.T) {
	app := emptyTestApp(t)

	t0 := time.Date(2024, 3, 1, 0, 0, 0, 0, time.UTC)

	fixtures := tthBaseFixtures(database.FixtureMap{
		"challenges": []map[string]any{
			{"id": "p", "type": "aggregate", "amount": "1", "active": true, "step_count": 1},
		},
		"sol_token_account_balance_changes": []map[string]any{
			{"signature": "reward_sig", "mint": tthWAudioMint, "owner": tthClaimablePDA, "account": tthUserABank, "change": 1000000, "balance": 1000000, "slot": 30, "block_timestamp": t0},
		},
		"sol_reward_disbursements": []map[string]any{
			{"signature": "reward_sig", "instruction_index": 0, "amount": 1000000, "slot": 30, "user_bank": tthUserABank, "challenge_id": "p", "specifier": "1", "recipient_eth_address": tthUserAWallet, "created_at": t0},
		},
	})
	database.Seed(app.pool.Replicas[0], fixtures)

	status, body := testGet(t, app, "/v1/users/"+trashid.MustEncodeHashID(1)+"/transactions/audio")
	assert.Equal(t, 200, status)
	jsonAssert(t, body, map[string]any{
		"data.0.transaction_type": "user_reward",
		"data.0.method":           "receive",
		"data.0.metadata":         "p", // challenge_id
	})
}

// TRENDING_REWARD: sol_reward_disbursements where the challenge type is 'trending'.
func TestVTokenTransactionsHistory_TrendingReward(t *testing.T) {
	app := emptyTestApp(t)

	t0 := time.Date(2024, 3, 1, 0, 0, 0, 0, time.UTC)

	fixtures := tthBaseFixtures(database.FixtureMap{
		"challenges": []map[string]any{
			{"id": "tt", "type": "trending", "amount": "1", "active": true, "step_count": 1},
		},
		"sol_token_account_balance_changes": []map[string]any{
			{"signature": "trending_sig", "mint": tthWAudioMint, "owner": tthClaimablePDA, "account": tthUserABank, "change": 5000000, "balance": 5000000, "slot": 40, "block_timestamp": t0},
		},
		"sol_reward_disbursements": []map[string]any{
			{"signature": "trending_sig", "instruction_index": 0, "amount": 5000000, "slot": 40, "user_bank": tthUserABank, "challenge_id": "tt", "specifier": "2024-W12", "recipient_eth_address": tthUserAWallet, "created_at": t0},
		},
	})
	database.Seed(app.pool.Replicas[0], fixtures)

	status, body := testGet(t, app, "/v1/users/"+trashid.MustEncodeHashID(1)+"/transactions/audio")
	assert.Equal(t, 200, status)
	jsonAssert(t, body, map[string]any{
		"data.0.transaction_type": "trending_reward",
		"data.0.method":           "receive",
		"data.0.metadata":         "tt",
	})
}

// Memo-tagged transfer types: when sol_transfer_memo_types carries a row for
// a transfer's signature, the view returns that type (withdrawal /
// prepare_withdrawal / internal_transfer) instead of bare `transfer`. Also
// asserts the tx_metadata override for `withdrawal` (returns to_account).
func TestVTokenTransactionsHistory_MemoTaggedTransfers(t *testing.T) {
	app := emptyTestApp(t)

	t0 := time.Date(2024, 5, 1, 0, 0, 0, 0, time.UTC)
	externalDest := "ExternalWithdrawDest____________________________"

	fixtures := tthBaseFixtures(database.FixtureMap{
		"sol_token_account_balance_changes": []map[string]any{
			// Three outbound transfers from user A's bank to varied destinations.
			{"signature": "withdraw_sig", "mint": tthWAudioMint, "owner": tthClaimablePDA, "account": tthUserABank, "change": -100, "balance": 900, "slot": 100, "block_timestamp": t0},
			{"signature": "prep_sig", "mint": tthWAudioMint, "owner": tthClaimablePDA, "account": tthUserABank, "change": -200, "balance": 700, "slot": 200, "block_timestamp": t0.Add(time.Hour)},
			{"signature": "intern_sig", "mint": tthWAudioMint, "owner": tthClaimablePDA, "account": tthUserABank, "change": -300, "balance": 400, "slot": 300, "block_timestamp": t0.Add(2 * time.Hour)},
		},
		"sol_claimable_account_transfers": []map[string]any{
			{"signature": "withdraw_sig", "instruction_index": 1, "amount": 100, "slot": 100, "from_account": tthUserABank, "to_account": externalDest, "sender_eth_address": tthUserAWallet},
			{"signature": "prep_sig", "instruction_index": 1, "amount": 200, "slot": 200, "from_account": tthUserABank, "to_account": externalDest, "sender_eth_address": tthUserAWallet},
			// Internal transfer counterpart is user B (a known bank). The memo
			// marker takes precedence over the tip branch.
			{"signature": "intern_sig", "instruction_index": 1, "amount": 300, "slot": 300, "from_account": tthUserABank, "to_account": tthUserBBank, "sender_eth_address": tthUserAWallet},
		},
		"sol_transfer_memo_types": []map[string]any{
			{"signature": "withdraw_sig", "instruction_index": 1, "slot": 100, "memo_type": "withdrawal"},
			{"signature": "prep_sig", "instruction_index": 1, "slot": 200, "memo_type": "prepare_withdrawal"},
			{"signature": "intern_sig", "instruction_index": 1, "slot": 300, "memo_type": "internal_transfer"},
		},
	})
	database.Seed(app.pool.Replicas[0], fixtures)

	// Default sort is transaction_date DESC.
	status, body := testGet(t, app, "/v1/users/"+trashid.MustEncodeHashID(1)+"/transactions/audio")
	assert.Equal(t, 200, status)
	jsonAssert(t, body, map[string]any{
		"data.#": 3,

		"data.0.transaction_type": "internal_transfer",
		"data.0.method":           "send",
		"data.0.change":           float64(300),

		"data.1.transaction_type": "prepare_withdrawal",
		"data.1.method":           "send",
		"data.1.change":           float64(200),

		"data.2.transaction_type": "withdrawal",
		"data.2.method":           "send",
		"data.2.change":           float64(100),
		// Withdrawal-specific tx_metadata: surfaces cat.to_account.
		"data.2.metadata": externalDest,
	})
}

// Recover-withdrawal markers are populated from the payment_router branch.
// The view should classify the balance change as `recover_withdrawal` even
// though there is no sol_claimable_account_transfers row for it.
func TestVTokenTransactionsHistory_RecoverWithdrawal(t *testing.T) {
	app := emptyTestApp(t)

	t0 := time.Date(2024, 5, 2, 0, 0, 0, 0, time.UTC)

	fixtures := tthBaseFixtures(database.FixtureMap{
		"sol_token_account_balance_changes": []map[string]any{
			{"signature": "recover_sig", "mint": tthWAudioMint, "owner": tthClaimablePDA, "account": tthUserABank, "change": 500, "balance": 500, "slot": 400, "block_timestamp": t0},
		},
		"sol_transfer_memo_types": []map[string]any{
			{"signature": "recover_sig", "instruction_index": 0, "slot": 400, "memo_type": "recover_withdrawal"},
		},
	})
	database.Seed(app.pool.Replicas[0], fixtures)

	status, body := testGet(t, app, "/v1/users/"+trashid.MustEncodeHashID(1)+"/transactions/audio")
	assert.Equal(t, 200, status)
	jsonAssert(t, body, map[string]any{
		"data.0.transaction_type": "recover_withdrawal",
		"data.0.method":           "receive",
		"data.0.change":           float64(500),
	})
}

// Mint isolation: the view must not surface rows from other mints when filtered
// to wAUDIO. Set up a USDC balance change on the same user_bank account and
// confirm /transactions/audio doesn't return it.
func TestVTokenTransactionsHistory_MintIsolation(t *testing.T) {
	app := emptyTestApp(t)

	usdcMint := "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v"
	t0 := time.Date(2024, 4, 1, 0, 0, 0, 0, time.UTC)

	fixtures := tthBaseFixtures(database.FixtureMap{
		"sol_claimable_accounts": []map[string]any{
			// Same user_bank account, different mint registration for USDC.
			// (in practice they'd be different PDAs; this only matters for the
			// view's join key.)
			{"signature": "claim_a_usdc", "instruction_index": 0, "slot": 1, "mint": usdcMint, "ethereum_address": tthUserAWallet, "account": tthUserABank + "_usdc"},
		},
		"sol_token_account_balance_changes": []map[string]any{
			{"signature": "usdc_sig", "mint": usdcMint, "owner": tthClaimablePDA, "account": tthUserABank + "_usdc", "change": 1000000, "balance": 1000000, "slot": 50, "block_timestamp": t0},
		},
	})
	// Re-seed the wAUDIO sol_claimable_accounts the base fixtures created.
	// FixtureMap entries are merged; need to combine.
	fixtures["sol_claimable_accounts"] = append(
		fixtures["sol_claimable_accounts"],
		map[string]any{"signature": "claim_a", "instruction_index": 1, "slot": 1, "mint": tthWAudioMint, "ethereum_address": tthUserAWallet, "account": tthUserABank},
	)
	database.Seed(app.pool.Replicas[0], fixtures)

	// The wAUDIO endpoint should return 0 rows since we only seeded a USDC row.
	status, body := testGet(t, app, "/v1/users/"+trashid.MustEncodeHashID(1)+"/transactions/audio")
	assert.Equal(t, 200, status)
	jsonAssert(t, body, map[string]any{
		"data.#": 0,
	})
}
