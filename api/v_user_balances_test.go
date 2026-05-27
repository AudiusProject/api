package api

import (
	"context"
	"testing"

	"api.audius.co/database"
	"github.com/stretchr/testify/assert"
)

// Tests for v_user_balances. The view sources:
//   * ETH side from eth_wallet_balances (primary + linked chain=eth wallets)
//   * wAUDIO side from sol_user_balances (pre-aggregated user_bank + linked sol
//     wallets, maintained by update_sol_user_balance_mint triggers)
//
// The associated_sol_wallets_balance column is always '0' — sol_user_balances
// collapses the legacy user_bank-vs-linked split into a single per-user/mint
// total. Downstream total_balance computations sum waudio +
// associated_sol_wallets_balance, so totals stay unchanged.

const (
	vubWAudioMint        = "9LzCMqDgTKYz9Drzqnpgee3SGa89up3a247ypMj2xrqM"
	vubUserPrimaryWallet = "0xubprimary"
	vubLinkedEthA        = "0xublinkedetha"
	vubLinkedEthB        = "0xublinkedethb"
	vubUserBank          = "VubUserBankPDA__________________________________"
	vubLinkedSolWallet   = "VubLinkedSolWallet______________________________"
	vubLinkedSolAccount  = "VubLinkedSolWalletWAudioTokenAcct_______________"
)

// row holds the v_user_balances columns we assert on.
type vUserBalanceRow struct {
	Balance                     string
	AssociatedWalletsBalance    string
	Waudio                      string
	AssociatedSolWalletsBalance string
}

func queryVUserBalances(t *testing.T, app *ApiServer, userID int) vUserBalanceRow {
	t.Helper()
	var r vUserBalanceRow
	err := app.pool.QueryRow(context.Background(), `
		SELECT balance, associated_wallets_balance, waudio, associated_sol_wallets_balance
		FROM v_user_balances
		WHERE user_id = $1
	`, userID).Scan(&r.Balance, &r.AssociatedWalletsBalance, &r.Waudio, &r.AssociatedSolWalletsBalance)
	assert.NoError(t, err)
	return r
}

// User with an ETH primary-wallet balance only.
func TestVUserBalances_EthPrimaryOnly(t *testing.T) {
	app := emptyTestApp(t)
	fixtures := database.FixtureMap{
		"users": []map[string]any{
			{"user_id": 1, "handle": "u1", "wallet": vubUserPrimaryWallet},
		},
		"eth_wallet_balances": []map[string]any{
			{"wallet": vubUserPrimaryWallet, "balance": "1000000000000000000"}, // 1 AUDIO in wei
		},
	}
	database.Seed(app.pool.Replicas[0], fixtures)

	r := queryVUserBalances(t, app, 1)
	assert.Equal(t, "1000000000000000000", r.Balance)
	assert.Equal(t, "0", r.AssociatedWalletsBalance)
	assert.Equal(t, "0", r.Waudio)
	assert.Equal(t, "0", r.AssociatedSolWalletsBalance)
}

// User with linked ETH wallets — view sums across them.
func TestVUserBalances_LinkedEthSum(t *testing.T) {
	app := emptyTestApp(t)
	fixtures := database.FixtureMap{
		"users": []map[string]any{
			{"user_id": 1, "handle": "u1", "wallet": vubUserPrimaryWallet},
		},
		"associated_wallets": []map[string]any{
			{"id": 1, "user_id": 1, "wallet": vubLinkedEthA, "chain": "eth", "blockhash": "h", "blocknumber": 101, "is_current": true, "is_delete": false},
			{"id": 2, "user_id": 1, "wallet": vubLinkedEthB, "chain": "eth", "blockhash": "h", "blocknumber": 101, "is_current": true, "is_delete": false},
		},
		"eth_wallet_balances": []map[string]any{
			{"wallet": vubUserPrimaryWallet, "balance": "100"},
			{"wallet": vubLinkedEthA, "balance": "200"},
			{"wallet": vubLinkedEthB, "balance": "300"},
		},
	}
	database.Seed(app.pool.Replicas[0], fixtures)

	r := queryVUserBalances(t, app, 1)
	assert.Equal(t, "100", r.Balance)
	assert.Equal(t, "500", r.AssociatedWalletsBalance) // 200 + 300
}

// User with wAUDIO on their user_bank PDA — surfaces under `waudio` (the
// sol_user_balances trigger sums user_bank + linked sol wallets into one row).
func TestVUserBalances_UserBankWAudio(t *testing.T) {
	app := emptyTestApp(t)
	fixtures := database.FixtureMap{
		"users": []map[string]any{
			{"user_id": 1, "handle": "u1", "wallet": vubUserPrimaryWallet},
		},
		"sol_claimable_accounts": []map[string]any{
			{"signature": "create_sig", "instruction_index": 0, "slot": 1, "mint": vubWAudioMint, "ethereum_address": vubUserPrimaryWallet, "account": vubUserBank},
		},
		"sol_token_account_balances": []map[string]any{
			{"account": vubUserBank, "owner": "claimable-pda", "mint": vubWAudioMint, "balance": 42000000000, "slot": 10},
		},
	}
	database.Seed(app.pool.Replicas[0], fixtures)

	r := queryVUserBalances(t, app, 1)
	assert.Equal(t, "42000000000", r.Waudio)
	// Always 0 — see file-level comment.
	assert.Equal(t, "0", r.AssociatedSolWalletsBalance)
}

// User with wAUDIO held by a linked Solana wallet — also surfaces under
// `waudio` since sol_user_balances rolls the linked-sol leg into the same row.
func TestVUserBalances_LinkedSolWAudio(t *testing.T) {
	app := emptyTestApp(t)
	fixtures := database.FixtureMap{
		"users": []map[string]any{
			{"user_id": 1, "handle": "u1", "wallet": vubUserPrimaryWallet},
		},
		"associated_wallets": []map[string]any{
			{"id": 1, "user_id": 1, "wallet": vubLinkedSolWallet, "chain": "sol", "blockhash": "h", "blocknumber": 101, "is_current": true, "is_delete": false},
		},
		"sol_token_account_balances": []map[string]any{
			{"account": vubLinkedSolAccount, "owner": vubLinkedSolWallet, "mint": vubWAudioMint, "balance": 7500000, "slot": 10},
		},
	}
	database.Seed(app.pool.Replicas[0], fixtures)

	r := queryVUserBalances(t, app, 1)
	assert.Equal(t, "7500000", r.Waudio)
	assert.Equal(t, "0", r.AssociatedSolWalletsBalance)
}

// All sources populated — ETH still splits primary vs linked; sol legs are
// combined into a single waudio total.
func TestVUserBalances_AllSources(t *testing.T) {
	app := emptyTestApp(t)
	fixtures := database.FixtureMap{
		"users": []map[string]any{
			{"user_id": 1, "handle": "u1", "wallet": vubUserPrimaryWallet},
		},
		"associated_wallets": []map[string]any{
			{"id": 1, "user_id": 1, "wallet": vubLinkedEthA, "chain": "eth", "blockhash": "h", "blocknumber": 101, "is_current": true, "is_delete": false},
			{"id": 2, "user_id": 1, "wallet": vubLinkedSolWallet, "chain": "sol", "blockhash": "h", "blocknumber": 101, "is_current": true, "is_delete": false},
		},
		"eth_wallet_balances": []map[string]any{
			{"wallet": vubUserPrimaryWallet, "balance": "1"},
			{"wallet": vubLinkedEthA, "balance": "2"},
		},
		"sol_claimable_accounts": []map[string]any{
			{"signature": "s", "instruction_index": 0, "slot": 1, "mint": vubWAudioMint, "ethereum_address": vubUserPrimaryWallet, "account": vubUserBank},
		},
		"sol_token_account_balances": []map[string]any{
			{"account": vubUserBank, "owner": "claimable-pda", "mint": vubWAudioMint, "balance": 3, "slot": 10},
			{"account": vubLinkedSolAccount, "owner": vubLinkedSolWallet, "mint": vubWAudioMint, "balance": 4, "slot": 10},
		},
	}
	database.Seed(app.pool.Replicas[0], fixtures)

	r := queryVUserBalances(t, app, 1)
	assert.Equal(t, "1", r.Balance)
	assert.Equal(t, "2", r.AssociatedWalletsBalance)
	assert.Equal(t, "7", r.Waudio, "user_bank + linked sol rolled into waudio") // 3 + 4
	assert.Equal(t, "0", r.AssociatedSolWalletsBalance)
}

// User with no on-chain balances anywhere — view still returns a row with
// zero balances (the LEFT JOINs return no match, COALESCE fills 0).
func TestVUserBalances_NoBalances(t *testing.T) {
	app := emptyTestApp(t)
	fixtures := database.FixtureMap{
		"users": []map[string]any{
			{"user_id": 1, "handle": "u1", "wallet": vubUserPrimaryWallet},
		},
	}
	database.Seed(app.pool.Replicas[0], fixtures)

	r := queryVUserBalances(t, app, 1)
	assert.Equal(t, "0", r.Balance)
	assert.Equal(t, "0", r.AssociatedWalletsBalance)
	assert.Equal(t, "0", r.Waudio)
	assert.Equal(t, "0", r.AssociatedSolWalletsBalance)
}

// Deleted associated_wallets must not contribute to associated_wallets_balance.
// (sol_user_balances triggers handle the sol-side delete case internally; this
// asserts the LATERAL on the ETH side honors the same filter.)
func TestVUserBalances_DeletedAssociatedWalletsExcluded(t *testing.T) {
	app := emptyTestApp(t)
	fixtures := database.FixtureMap{
		"users": []map[string]any{
			{"user_id": 1, "handle": "u1", "wallet": vubUserPrimaryWallet},
		},
		"associated_wallets": []map[string]any{
			{"id": 1, "user_id": 1, "wallet": vubLinkedEthA, "chain": "eth", "blockhash": "h", "blocknumber": 101, "is_current": true, "is_delete": true}, // deleted
		},
		"eth_wallet_balances": []map[string]any{
			{"wallet": vubLinkedEthA, "balance": "999"},
		},
	}
	database.Seed(app.pool.Replicas[0], fixtures)

	r := queryVUserBalances(t, app, 1)
	assert.Equal(t, "0", r.AssociatedWalletsBalance, "deleted associated_wallets should not contribute")
}
