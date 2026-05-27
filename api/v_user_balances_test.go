package api

import (
	"context"
	"testing"

	"api.audius.co/database"
	"github.com/stretchr/testify/assert"
)

// Tests for v_user_balances. The view is a drop-in replacement for the legacy
// user_balances table, pulling ETH-side balances from eth_wallet_balances and
// wAUDIO from sol_token_account_balances via the same join chains that
// update_sol_user_balance_mint uses.

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

// User with wAUDIO on their user_bank PDA.
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
	assert.Equal(t, "0", r.AssociatedSolWalletsBalance)
}

// User with wAUDIO held by a linked Solana wallet.
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
	assert.Equal(t, "0", r.Waudio)
	assert.Equal(t, "7500000", r.AssociatedSolWalletsBalance)
}

// User with all four sources populated — view returns each leg independently.
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
	assert.Equal(t, "3", r.Waudio)
	assert.Equal(t, "4", r.AssociatedSolWalletsBalance)
}

// User with no on-chain balances anywhere — view still returns a row with
// zero balances (the LEFT JOIN LATERALs return nothing, COALESCE fills 0).
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

// Deleted associated wallets are excluded from sums (mirrors update_sol_user_balance.sql).
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
