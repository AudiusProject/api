package api

import (
	"context"
	"testing"

	"api.audius.co/database"
	"github.com/stretchr/testify/assert"
)

// Tests for v_user_balances. The view exposes one total per network per user:
//   * eth_balance — SUM of eth_wallet_balances over the user's primary wallet
//     plus all chain=eth associated_wallets (current, not deleted), in wei.
//   * sol_balance — sol_user_balances for the wAUDIO mint, already pre-aggregated
//     across user_bank PDAs + linked Solana wallets by the
//     handle_sol_claimable_accounts / update_sol_user_balance triggers, in
//     wAUDIO base units (8 decimals).

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
	EthBalance string
	SolBalance string
}

func queryVUserBalances(t *testing.T, app *ApiServer, userID int) vUserBalanceRow {
	t.Helper()
	var r vUserBalanceRow
	err := app.pool.QueryRow(context.Background(), `
		SELECT eth_balance, sol_balance
		FROM v_user_balances
		WHERE user_id = $1
	`, userID).Scan(&r.EthBalance, &r.SolBalance)
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
	assert.Equal(t, "1000000000000000000", r.EthBalance)
	assert.Equal(t, "0", r.SolBalance)
}

// User with linked ETH wallets — view sums primary + linked into eth_balance.
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
	assert.Equal(t, "600", r.EthBalance) // 100 + 200 + 300
	assert.Equal(t, "0", r.SolBalance)
}

// User with wAUDIO on their user_bank PDA — surfaces under sol_balance (the
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
	assert.Equal(t, "0", r.EthBalance)
	assert.Equal(t, "42000000000", r.SolBalance)
}

// User with wAUDIO held by a linked Solana wallet — also surfaces under
// sol_balance since sol_user_balances rolls the linked-sol leg into the same row.
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
	assert.Equal(t, "0", r.EthBalance)
	assert.Equal(t, "7500000", r.SolBalance)
}

// All sources populated — eth_balance is the sum of primary + linked eth,
// sol_balance is the sum of user_bank + linked sol.
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
	assert.Equal(t, "3", r.EthBalance, "primary + linked eth")          // 1 + 2
	assert.Equal(t, "7", r.SolBalance, "user_bank + linked sol rolled") // 3 + 4
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
	assert.Equal(t, "0", r.EthBalance)
	assert.Equal(t, "0", r.SolBalance)
}

// Deleted associated_wallets must not contribute to eth_balance.
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
	assert.Equal(t, "0", r.EthBalance, "deleted associated_wallets should not contribute")
}
