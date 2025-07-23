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
