package indexer

import (
	"testing"
	"time"

	"bridgerton.audius.co/database"
	"github.com/rpcpool/yellowstone-grpc/examples/golang/proto"
	"github.com/test-go/testify/assert"
)

// Ensures the database matches the expected schema for the inserts
func TestInserts(t *testing.T) {
	pool := database.CreateTestDatabase(t, "test_solana_indexer")
	defer pool.Close()

	err := insertBalanceChange(t.Context(), pool, balanceChangeRow{
		account: "account1",
		balanceChange: balanceChange{
			Owner:            "owner1",
			Mint:             "mint1",
			PreTokenBalance:  1000,
			PostTokenBalance: 2000,
			Change:           1000,
		},
		signature:      "signature1",
		slot:           12345,
		blockTimestamp: time.Now(),
	})
	assert.NoError(t, err, "failed to insert balance change")

	err = insertClaimableAccount(t.Context(), pool, claimableAccountsRow{
		signature:        "signature2",
		instructionIndex: 0,
		slot:             12345,
		mint:             "mint2",
		ethereumAddress:  "0x1234567890abcdef1234567890abcdef",
		account:          "account2",
	})
	assert.NoError(t, err, "failed to insert claimable account")

	err = insertClaimableAccountTransfer(t.Context(), pool, claimableAccountTransfersRow{
		signature:        "signature3",
		instructionIndex: 0,
		amount:           1000,
		slot:             12345,
		fromAccount:      "fromAccount2",
		toAccount:        "toAccount2",
		senderEthAddress: "0xabcdef1234567890abcdef1234567890",
	})
	assert.NoError(t, err, "failed to insert claimable account transfer")

	err = insertPayment(t.Context(), pool, paymentRow{
		signature:        "signature4",
		instructionIndex: 0,
		amount:           5000,
		slot:             12345,
		routeIndex:       0,
		toAccount:        "toAccount3",
	})
	assert.NoError(t, err, "failed to insert payment router transaction")

	err = insertPurchase(t.Context(), pool, purchaseRow{
		signature:        "signature5",
		instructionIndex: 0,
		amount:           10000,
		slot:             12345,
		fromAccount:      "fromAccount3",
		parsedPurchaseMemo: parsedPurchaseMemo{
			ContentId:             123,
			ContentType:           "track",
			ValidAfterBlocknumber: 12345678,
			BuyerUserId:           1,
			AccessType:            "stream",
		},
		parsedLocationMemo: parsedLocationMemo{
			City:    "San Francisco",
			Country: "USA",
			Region:  "California",
		},
		isValid: nil,
	})
	assert.NoError(t, err, "failed to insert purchase")

	err = insertRewardDisbursement(t.Context(), pool, rewardDisbursementsRow{
		signature:        "signature6",
		instructionIndex: 0,
		amount:           2000,
		slot:             12345,
		userBank:         "userBank1",
		challengeId:      "challenge1",
		specifier:        "specifier1",
	})
	assert.NoError(t, err, "failed to insert reward disbursement")

	req := proto.SubscribeRequest{}
	id, err := insertCheckpointStart(t.Context(), pool, 100, &req)
	assert.NoError(t, err, "failed to insert checkpoint start")
	assert.NotEmpty(t, id, "checkpoint ID should not be empty")

	err = updateCheckpoint(t.Context(), pool, id, 201)
	assert.NoError(t, err, "failed to update checkpoint")

	slot, err := getCheckpointSlot(t.Context(), pool, &req)
	assert.NoError(t, err, "failed to get checkpoint slot")
	assert.Equal(t, uint64(201), slot, "checkpoint slot should match updated value")

	id2, err := insertBackfillCheckpoint(t.Context(), pool, 100, 200, "foo")
	assert.NoError(t, err, "failed to insert backfill checkpoint")
	assert.NotEmpty(t, id2, "backfill checkpoint ID should not be empty")
}

func TestInsertBalanceChangeTriggers(t *testing.T) {
	pool := database.CreateTestDatabase(t, "test_solana_indexer")
	defer pool.Close()

	database.Seed(pool, database.FixtureMap{
		"users": []map[string]any{
			{
				"user_id": 1,
				"wallet":  "0x1234567890abcdef1234567890abcdef",
				"handle":  "testuser",
			},
		},
		"sol_claimable_accounts": []map[string]any{
			{
				"signature":        "signature1",
				"account":          "claimable-account",
				"ethereum_address": "0x1234567890abcdef1234567890abcdef",
				"mint":             "mint1",
			},
		},
		"associated_wallets": []map[string]any{
			{
				"id":      1,
				"user_id": 1,
				"wallet":  "owner1",
				"chain":   "sol",
			},
			{
				"id":      2,
				"user_id": 1,
				"wallet":  "owner2",
				"chain":   "sol",
			},
		},
	})

	{
		// Insert a claimable token account balance change and verify
		// the token balance updates and the user balance updates
		err := insertBalanceChange(t.Context(), pool, balanceChangeRow{
			account: "claimable-account",
			balanceChange: balanceChange{
				Owner:            "claimablePda",
				Mint:             "mint1",
				PreTokenBalance:  0,
				PostTokenBalance: 2000,
				Change:           2000,
			},
			signature:      "signature2",
			slot:           10002,
			blockTimestamp: time.Now(),
		})
		assert.NoError(t, err, "failed to insert balance change")

		// Verify that the balance was updated correctly
		var balance int64
		err = pool.QueryRow(t.Context(),
			"SELECT balance FROM sol_token_account_balances WHERE account = $1",
			"claimable-account",
		).Scan(&balance)
		assert.NoError(t, err, "failed to query balance")
		assert.Equal(t, int64(2000), balance, "balance should be updated to 2000")

		// Verify that the user's balance was updated correctly
		var userBalance int64
		err = pool.QueryRow(t.Context(),
			"SELECT balance FROM sol_user_balances WHERE user_id = $1 AND mint = $2",
			1, "mint1",
		).Scan(&userBalance)
		assert.NoError(t, err, "failed to query user balance")
		assert.Equal(t, int64(2000), userBalance, "user balance should be updated to 2000")
	}

	{
		// Insert an associated wallet balance change and verify the
		// token balance updates and the user balance updates
		err := insertBalanceChange(t.Context(), pool, balanceChangeRow{
			account: "account1",
			balanceChange: balanceChange{
				Owner:            "owner1",
				Mint:             "mint1",
				PreTokenBalance:  0,
				PostTokenBalance: 3000,
				Change:           3000,
			},
			signature:      "signature3",
			slot:           10003,
			blockTimestamp: time.Now(),
		})
		assert.NoError(t, err, "failed to insert balance change")

		// Verify that the token account balance was updated correctly
		var balance int64
		err = pool.QueryRow(t.Context(),
			"SELECT balance FROM sol_token_account_balances WHERE account = $1",
			"account1",
		).Scan(&balance)
		assert.NoError(t, err, "failed to query balance")
		assert.Equal(t, int64(3000), balance, "balance should be updated to 3000")

		// Verify that the user's balance was updated correctly
		var userBalance int64
		err = pool.QueryRow(t.Context(),
			"SELECT balance FROM sol_user_balances WHERE user_id = $1 AND mint = $2",
			1, "mint1",
		).Scan(&userBalance)
		assert.NoError(t, err, "failed to query user balance")
		assert.Equal(t, int64(5000), userBalance, "user balance should be updated to 5000")
	}

	{
		// Insert another associated wallet balance change and verify the
		// token balance updates and the user balance updates
		err := insertBalanceChange(t.Context(), pool, balanceChangeRow{
			account: "account2",
			balanceChange: balanceChange{
				Owner:            "owner2",
				Mint:             "mint1",
				PreTokenBalance:  0,
				PostTokenBalance: 5000,
				Change:           5000,
			},
			signature:      "signature4",
			slot:           10004,
			blockTimestamp: time.Now(),
		})
		assert.NoError(t, err, "failed to insert balance change")

		// Verify that the token account balance was updated correctly
		var balance int64
		err = pool.QueryRow(t.Context(),
			"SELECT balance FROM sol_token_account_balances WHERE account = $1",
			"account2",
		).Scan(&balance)
		assert.NoError(t, err, "failed to query balance")
		assert.Equal(t, int64(5000), balance, "balance should be updated to 5000")

		// Verify that the user's balance was updated correctly
		var userBalance int64
		err = pool.QueryRow(t.Context(),
			"SELECT balance FROM sol_user_balances WHERE user_id = $1 AND mint = $2",
			1, "mint1",
		).Scan(&userBalance)
		assert.NoError(t, err, "failed to query user balance")
		assert.Equal(t, int64(10000), userBalance, "user balance should be updated to 10000")
	}

	{
		// Insert a negative claimable token account balance change and verify
		// the token balance updates and the user balance updates
		err := insertBalanceChange(t.Context(), pool, balanceChangeRow{
			account: "claimable-account",
			balanceChange: balanceChange{
				Owner:            "claimablePda",
				Mint:             "mint1",
				PreTokenBalance:  2000,
				PostTokenBalance: 0,
				Change:           -2000,
			},
			signature:      "signature5",
			slot:           10005,
			blockTimestamp: time.Now(),
		})
		assert.NoError(t, err, "failed to insert balance change")

		// Verify that the balance was updated correctly
		var balance int64
		err = pool.QueryRow(t.Context(),
			"SELECT balance FROM sol_token_account_balances WHERE account = $1",
			"claimable-account",
		).Scan(&balance)
		assert.NoError(t, err, "failed to query balance")
		assert.Equal(t, int64(0), balance, "balance should be updated to 0")

		// Verify that the user's balance was updated correctly
		var userBalance int64
		err = pool.QueryRow(t.Context(),
			"SELECT balance FROM sol_user_balances WHERE user_id = $1 AND mint = $2",
			1, "mint1",
		).Scan(&userBalance)
		assert.NoError(t, err, "failed to query user balance")
		assert.Equal(t, int64(8000), userBalance, "user balance should be updated to 8000")
	}

	{
		// Insert a balance change, then later associate it and verify the
		// newly associated balance is updated correctly
		err := insertBalanceChange(t.Context(), pool, balanceChangeRow{
			account: "account3",
			balanceChange: balanceChange{
				Owner:            "owner3",
				Mint:             "mint1",
				PreTokenBalance:  0,
				PostTokenBalance: 1000,
				Change:           1000,
			},
			signature:      "signature5",
			slot:           10006,
			blockTimestamp: time.Now(),
		})
		assert.NoError(t, err, "failed to insert balance change")

		// Verify that the token account balance was updated correctly
		var balance int64
		err = pool.QueryRow(t.Context(),
			"SELECT balance FROM sol_token_account_balances WHERE account = $1",
			"account3",
		).Scan(&balance)
		assert.NoError(t, err, "failed to query balance")
		assert.Equal(t, int64(1000), balance, "balance should be updated to 1000")

		// Verify that the user's balance was not updated yet
		var userBalance int64
		err = pool.QueryRow(t.Context(),
			"SELECT balance FROM sol_user_balances WHERE user_id = $1 AND mint = $2",
			1, "mint1",
		).Scan(&userBalance)
		assert.NoError(t, err, "failed to query user balance")
		assert.Equal(t, int64(8000), userBalance, "user balance should still be 8000")

		// Now associate the wallet and verify the user balance is updated
		_, err = pool.Exec(t.Context(),
			`INSERT INTO associated_wallets
				(id, user_id, wallet, chain, blockhash, blocknumber, is_current, is_delete)
			VALUES
				($1, $2, $3, $4, $5, $6, $7, $8)
			`,
			3, 1, "owner3", "sol", "blockhash3", 101, true, false,
		)
		assert.NoError(t, err, "failed to insert associated wallet")
		err = pool.QueryRow(t.Context(),
			"SELECT balance FROM sol_user_balances WHERE user_id = $1 AND mint = $2",
			1, "mint1",
		).Scan(&userBalance)
		assert.NoError(t, err, "failed to query user balance after association")
		assert.Equal(t, int64(9000), userBalance, "user balance should be updated to 9000 after association")
	}

	{
		// Unassociate a wallet and verify the user balance is updated
		_, err := pool.Exec(t.Context(),
			"DELETE FROM associated_wallets WHERE user_id = $1 AND wallet = $2 AND chain = $3",
			1, "owner2", "sol",
		)
		assert.NoError(t, err, "failed to unassociate wallet")

		var userBalance int64
		err = pool.QueryRow(t.Context(),
			"SELECT balance FROM sol_user_balances WHERE user_id = $1 AND mint = $2",
			1, "mint1",
		).Scan(&userBalance)
		assert.NoError(t, err, "failed to query user balance after unassociation")
		assert.Equal(t, int64(4000), userBalance, "user balance should be updated to 4000 after unassociation")
	}

	{
		// Unassociate a wallet and verify the user balance is updated
		_, err := pool.Exec(t.Context(),
			"UPDATE associated_wallets SET is_current = $1, is_delete = $2 WHERE user_id = $3 AND wallet = $4 AND chain = $5",
			false, true, 1, "owner1", "sol",
		)
		assert.NoError(t, err, "failed to unassociate wallet")

		var userBalance int64
		err = pool.QueryRow(t.Context(),
			"SELECT balance FROM sol_user_balances WHERE user_id = $1 AND mint = $2",
			1, "mint1",
		).Scan(&userBalance)
		assert.NoError(t, err, "failed to query user balance after unassociation")
		assert.Equal(t, int64(1000), userBalance, "user balance should be updated to 1000 after unassociation")
	}

	{
		// Verify that adding a claimable account when the balance change
		// is already present still updates the user balance correctly
		err := insertBalanceChange(t.Context(), pool, balanceChangeRow{
			account: "claimable-account-2",
			balanceChange: balanceChange{
				Owner:            "claimablePda",
				Mint:             "mint2",
				PreTokenBalance:  0,
				PostTokenBalance: 5000,
				Change:           5000,
			},
			signature:      "signature6",
			slot:           10007,
			blockTimestamp: time.Now(),
		})
		assert.NoError(t, err, "failed to insert balance change for claimable account")

		// Verify that the balance was updated correctly
		var balance int64
		err = pool.QueryRow(t.Context(),
			"SELECT balance FROM sol_token_account_balances WHERE account = $1",
			"claimable-account-2",
		).Scan(&balance)
		assert.NoError(t, err, "failed to query balance for claimable account")
		assert.Equal(t, int64(5000), balance, "balance should be updated to 5000 for claimable account")

		// Verify that the user's balance was not updated yet
		var userBalance int64
		err = pool.QueryRow(t.Context(),
			"SELECT balance FROM sol_user_balances WHERE user_id = $1 AND mint = $2",
			1, "mint2",
		).Scan(&userBalance)
		assert.Error(t, err, "expect no rows")
		assert.Equal(t, int64(0), userBalance, "user balance should not be updated yet for claimable account 2")

		// Now insert the claimable account and verify the user balance is updated
		err = insertClaimableAccount(t.Context(), pool, claimableAccountsRow{
			signature:        "signature7",
			instructionIndex: 0,
			slot:             10008,
			mint:             "mint2",
			ethereumAddress:  "0x1234567890abcdef1234567890abcdef",
			account:          "claimable-account-2",
		})
		assert.NoError(t, err, "failed to insert claimable account for claimable account 2")
		err = pool.QueryRow(t.Context(),
			"SELECT balance FROM sol_user_balances WHERE user_id = $1 AND mint = $2",
			1, "mint2",
		).Scan(&userBalance)
		assert.NoError(t, err, "failed to query user balance after inserting claimable account")
		assert.Equal(t, int64(5000), userBalance, "user balance should be updated to 5000 after inserting claimable account 2")
	}
}
