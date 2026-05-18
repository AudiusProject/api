package api

import (
	"testing"
	"time"

	"api.audius.co/database"
	"api.audius.co/trashid"
	"github.com/stretchr/testify/assert"
)

// These tests pin down the v_usdc_purchases splits[*].user_id resolution
// rules, which are easy to get wrong because they need to look at the seller's
// payout wallet *at the time of purchase*, not the current one. The route's
// regression history: the first cut joined users.spl_usdc_payout_wallet
// directly, which silently broke for any seller who ever changed payout
// wallets after a sale.

const buyerWallet = "0x7d273271690538cf855e5b3002a0dd8c154bb060"

// Scenario: seller changes payout wallet AFTER a purchase.
// sol_payments.to_account is the on-chain destination (immutable, equal to
// the seller's payout wallet at the time of purchase). users.spl_usdc_payout_wallet
// now points at the new wallet. The view must still resolve the historical
// split to the seller via user_payout_wallet_history.
func TestVUsdcPurchasesSplits_PayoutWalletChangedAfterPurchase(t *testing.T) {
	app := emptyTestApp(t)
	buyerID := 1
	sellerID := 2

	purchaseTime := time.Date(2024, 1, 15, 12, 0, 0, 0, time.UTC)
	walletAtPurchase := "SellerWalletAtPurchase"
	walletAfterChange := "SellerWalletAfterChange"

	fixtures := database.FixtureMap{
		"blocks": []map[string]any{
			{"blockhash": "block_initial", "parenthash": "0", "number": 200},
			{"blockhash": "block_change", "parenthash": "block_initial", "number": 300},
		},
		"users": []map[string]any{
			{"user_id": buyerID, "handle": "buyer", "wallet": buyerWallet},
			// users.spl_usdc_payout_wallet is the *current* (post-change) wallet.
			{"user_id": sellerID, "handle": "seller", "wallet": "0xseller", "spl_usdc_payout_wallet": walletAfterChange},
		},
		"tracks": []map[string]any{
			{"track_id": 1, "title": "song", "owner_id": sellerID},
		},
		"user_payout_wallet_history": []map[string]any{
			{"user_id": sellerID, "spl_usdc_payout_wallet": walletAtPurchase, "blocknumber": 200, "block_timestamp": purchaseTime.AddDate(0, -1, 0)},
			// Wallet change happened a month AFTER the purchase, at a different block.
			{"user_id": sellerID, "spl_usdc_payout_wallet": walletAfterChange, "blocknumber": 300, "block_timestamp": purchaseTime.AddDate(0, 1, 0)},
		},
		"sol_purchases": []map[string]any{
			{"signature": "sig1", "instruction_index": 0, "buyer_user_id": buyerID, "amount": 1000000, "content_type": "track", "content_id": 1, "created_at": purchaseTime, "is_valid": true},
		},
		"sol_payments": []map[string]any{
			// Payment landed at the seller's then-current payout wallet.
			{"signature": "sig1", "instruction_index": 0, "route_index": 0, "to_account": walletAtPurchase, "amount": 1000000, "slot": 101},
		},
	}
	database.Seed(app.pool.Replicas[0], fixtures)

	status, body := testGetWithWallet(t, app, "/v1/users/"+trashid.MustEncodeHashID(buyerID)+"/purchases", buyerWallet)
	assert.Equal(t, 200, status)
	jsonAssert(t, body, map[string]any{
		"data.#":                        1,
		"data.0.splits.0.user_id":       sellerID,
		"data.0.splits.0.payout_wallet": walletAtPurchase,
	})
}

// Scenario: seller has three historical payout wallets. The view should pick
// the one that was current at purchase time, not the most recent.
func TestVUsdcPurchasesSplits_PicksHistoricalWalletAtPurchaseTime(t *testing.T) {
	app := emptyTestApp(t)
	buyerID := 1
	sellerID := 2

	t0 := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	t1 := time.Date(2024, 4, 1, 0, 0, 0, 0, time.UTC)
	t2 := time.Date(2024, 7, 1, 0, 0, 0, 0, time.UTC)
	purchaseTime := time.Date(2024, 5, 15, 0, 0, 0, 0, time.UTC) // between t1 and t2

	fixtures := database.FixtureMap{
		"blocks": []map[string]any{
			{"blockhash": "block_a", "parenthash": "0", "number": 200},
			{"blockhash": "block_b", "parenthash": "block_a", "number": 300},
			{"blockhash": "block_c", "parenthash": "block_b", "number": 400},
		},
		"users": []map[string]any{
			{"user_id": buyerID, "handle": "buyer", "wallet": buyerWallet},
			{"user_id": sellerID, "handle": "seller", "wallet": "0xseller", "spl_usdc_payout_wallet": "WalletC"},
		},
		"tracks": []map[string]any{
			{"track_id": 1, "title": "song", "owner_id": sellerID},
		},
		"user_payout_wallet_history": []map[string]any{
			{"user_id": sellerID, "spl_usdc_payout_wallet": "WalletA", "blocknumber": 200, "block_timestamp": t0},
			{"user_id": sellerID, "spl_usdc_payout_wallet": "WalletB", "blocknumber": 300, "block_timestamp": t1},
			{"user_id": sellerID, "spl_usdc_payout_wallet": "WalletC", "blocknumber": 400, "block_timestamp": t2},
		},
		"sol_purchases": []map[string]any{
			{"signature": "sig1", "instruction_index": 0, "buyer_user_id": buyerID, "amount": 1000000, "content_type": "track", "content_id": 1, "created_at": purchaseTime, "is_valid": true},
		},
		"sol_payments": []map[string]any{
			// Payment landed at the wallet that was current at purchase time (WalletB).
			{"signature": "sig1", "instruction_index": 0, "route_index": 0, "to_account": "WalletB", "amount": 1000000, "slot": 101},
		},
	}
	database.Seed(app.pool.Replicas[0], fixtures)

	status, body := testGetWithWallet(t, app, "/v1/users/"+trashid.MustEncodeHashID(buyerID)+"/purchases", buyerWallet)
	assert.Equal(t, 200, status)
	jsonAssert(t, body, map[string]any{
		"data.0.splits.0.user_id":       sellerID,
		"data.0.splits.0.payout_wallet": "WalletB",
	})
}

// Scenario: seller never set a custom payout wallet (no user_payout_wallet_history
// rows). The payment lands at their USDC user-bank PDA. The view falls back to
// sol_claimable_accounts -> users.wallet for resolution.
func TestVUsdcPurchasesSplits_FallsBackToUserBankWhenNoPayoutHistory(t *testing.T) {
	app := emptyTestApp(t)
	buyerID := 1
	sellerID := 2
	sellerEthWallet := "0xseller_eth_wallet"
	sellerUserBank := "SellerUSDCUserBankPDA"

	fixtures := database.FixtureMap{
		"users": []map[string]any{
			{"user_id": buyerID, "handle": "buyer", "wallet": buyerWallet},
			// No spl_usdc_payout_wallet set; no user_payout_wallet_history rows.
			{"user_id": sellerID, "handle": "seller", "wallet": sellerEthWallet},
		},
		"tracks": []map[string]any{
			{"track_id": 1, "title": "song", "owner_id": sellerID},
		},
		"sol_claimable_accounts": []map[string]any{
			{
				"signature":         "create_sig",
				"instruction_index": 0,
				"slot":              1,
				"mint":              "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v",
				"ethereum_address":  sellerEthWallet,
				"account":           sellerUserBank,
			},
		},
		"sol_purchases": []map[string]any{
			{"signature": "sig1", "instruction_index": 0, "buyer_user_id": buyerID, "amount": 1000000, "content_type": "track", "content_id": 1, "created_at": time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC), "is_valid": true},
		},
		"sol_payments": []map[string]any{
			{"signature": "sig1", "instruction_index": 0, "route_index": 0, "to_account": sellerUserBank, "amount": 1000000, "slot": 101},
		},
	}
	database.Seed(app.pool.Replicas[0], fixtures)

	status, body := testGetWithWallet(t, app, "/v1/users/"+trashid.MustEncodeHashID(buyerID)+"/purchases", buyerWallet)
	assert.Equal(t, 200, status)
	jsonAssert(t, body, map[string]any{
		"data.0.splits.0.user_id":       sellerID,
		"data.0.splits.0.payout_wallet": sellerUserBank,
	})
}

// Scenario: a split goes to a wallet that doesn't map to any user (the network
// share / staking bridge). splits[*].user_id should be null, not an arbitrary
// match from current users.spl_usdc_payout_wallet.
func TestVUsdcPurchasesSplits_NetworkShareResolvesToNull(t *testing.T) {
	app := emptyTestApp(t)
	buyerID := 1
	sellerID := 2

	fixtures := database.FixtureMap{
		"users": []map[string]any{
			{"user_id": buyerID, "handle": "buyer", "wallet": buyerWallet},
			{"user_id": sellerID, "handle": "seller", "wallet": "0xseller", "spl_usdc_payout_wallet": "SellerPayout"},
		},
		"tracks": []map[string]any{
			{"track_id": 1, "title": "song", "owner_id": sellerID},
		},
		"blocks": []map[string]any{
			{"blockhash": "block_payout_set", "parenthash": "0", "number": 200},
		},
		"user_payout_wallet_history": []map[string]any{
			{"user_id": sellerID, "spl_usdc_payout_wallet": "SellerPayout", "blocknumber": 200, "block_timestamp": time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)},
		},
		"sol_purchases": []map[string]any{
			{"signature": "sig1", "instruction_index": 0, "buyer_user_id": buyerID, "amount": 1000000, "content_type": "track", "content_id": 1, "created_at": time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC), "is_valid": true},
		},
		"sol_payments": []map[string]any{
			{"signature": "sig1", "instruction_index": 0, "route_index": 0, "to_account": "SellerPayout", "amount": 900000, "slot": 101},
			// 10% goes to an unowned/network wallet.
			{"signature": "sig1", "instruction_index": 0, "route_index": 1, "to_account": "StakingBridgeWallet", "amount": 100000, "slot": 101},
		},
	}
	database.Seed(app.pool.Replicas[0], fixtures)

	status, body := testGetWithWallet(t, app, "/v1/users/"+trashid.MustEncodeHashID(buyerID)+"/purchases", buyerWallet)
	assert.Equal(t, 200, status)
	jsonAssert(t, body, map[string]any{
		"data.0.splits.0.user_id":       sellerID,
		"data.0.splits.0.payout_wallet": "SellerPayout",
		"data.0.splits.1.user_id":       nil,
		"data.0.splits.1.payout_wallet": "StakingBridgeWallet",
	})
}
