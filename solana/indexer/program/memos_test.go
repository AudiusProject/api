package program

import (
	"testing"

	"github.com/gagliardetto/solana-go"
	"github.com/test-go/testify/assert"
)

// makeMemoOnlyTx constructs a minimal solana.Transaction whose only
// instruction is a memo with the given payload, so the scanner helpers can be
// exercised without pulling in a real on-chain fixture.
func makeMemoOnlyTx(payload string) *solana.Transaction {
	tx := &solana.Transaction{}
	tx.Message.AccountKeys = solana.PublicKeySlice{solana.MemoProgramID}
	tx.Message.Instructions = []solana.CompiledInstruction{
		{
			ProgramIDIndex: 0,
			Data:           []byte(payload),
		},
	}
	return tx
}

// makeJupiterTouchingTx returns a transaction whose account_keys include the
// Jupiter V6 router — the prepare-withdrawal-by-program fallback should fire
// even without a "Prepare Withdrawal" memo.
func makeJupiterTouchingTx() *solana.Transaction {
	tx := &solana.Transaction{}
	tx.Message.AccountKeys = solana.PublicKeySlice{JUPITER_V6_PROGRAM_ID}
	return tx
}

func TestParsePurchaseMemo(t *testing.T) {
	// happy case
	expected := parsedPurchaseMemo{
		ContentType:           "track",
		ContentId:             1,
		ValidAfterBlocknumber: 2,
		BuyerUserId:           3,
		AccessType:            "download",
	}
	parsed, err := parsePurchaseMemo([]byte("track:1:2:3:download"))
	assert.NoError(t, err)
	assert.Equal(t, expected, parsed)

	// errors
	_, err = parsePurchaseMemo([]byte("not:purchase"))
	assert.EqualError(t, err, "not a purchase memo")
	_, err = parsePurchaseMemo([]byte("track:foo:2:3:download"))
	assert.EqualError(t, err, "failed to parse contentId: strconv.Atoi: parsing \"foo\": invalid syntax")
	_, err = parsePurchaseMemo([]byte("track:1:foo:3:download"))
	assert.EqualError(t, err, "failed to parse validAfterBlocknumber: strconv.Atoi: parsing \"foo\": invalid syntax")
	_, err = parsePurchaseMemo([]byte("track:1:2:foo:download"))
	assert.EqualError(t, err, "failed to parse buyerUserId: strconv.Atoi: parsing \"foo\": invalid syntax")
}

func TestParseTransferTypeMemo(t *testing.T) {
	cases := []struct {
		memo string
		want TransferType
	}{
		{"Withdrawal", TransferTypeWithdrawal},
		{"Prepare Withdrawal", TransferTypePrepareWithdrawal},
		{"Internal Transfer", TransferTypeInternalTransfer},
		{"Recover Withdrawal", TransferTypeRecoverWithdrawal},
		{"withdrawal", TransferTypeUnknown},   // case-sensitive
		{"track:1:2:3:download", TransferTypeUnknown},
		{"", TransferTypeUnknown},
	}
	for _, c := range cases {
		assert.Equal(t, c.want, parseTransferTypeMemo([]byte(c.memo)), c.memo)
	}
}

func TestFindTransferTypeMemo(t *testing.T) {
	// A memo-only tx with a recognized payload should be classified.
	assert.Equal(t, TransferTypeWithdrawal, findTransferTypeMemo(makeMemoOnlyTx("Withdrawal")))
	assert.Equal(t, TransferTypeInternalTransfer, findTransferTypeMemo(makeMemoOnlyTx("Internal Transfer")))

	// Unrecognized payload returns Unknown.
	assert.Equal(t, TransferTypeUnknown, findTransferTypeMemo(makeMemoOnlyTx("Hello")))

	// Empty tx returns Unknown.
	assert.Equal(t, TransferTypeUnknown, findTransferTypeMemo(&solana.Transaction{}))
}

func TestTransactionTouchesJupiter(t *testing.T) {
	assert.True(t, transactionTouchesJupiter(makeJupiterTouchingTx()))
	assert.False(t, transactionTouchesJupiter(makeMemoOnlyTx("Withdrawal")))
	assert.False(t, transactionTouchesJupiter(&solana.Transaction{}))
}

func TestTransferTypeString(t *testing.T) {
	// String values must match the legacy `transaction_type` strings the API
	// returns, since the view inserts them directly into the `memo_type` column.
	assert.Equal(t, "withdrawal", TransferTypeWithdrawal.String())
	assert.Equal(t, "prepare_withdrawal", TransferTypePrepareWithdrawal.String())
	assert.Equal(t, "internal_transfer", TransferTypeInternalTransfer.String())
	assert.Equal(t, "recover_withdrawal", TransferTypeRecoverWithdrawal.String())
}

func TestParseLocationMemo(t *testing.T) {
	// happy case
	expected := parsedLocationMemo{
		City:    "Minneapolis",
		Region:  "MN",
		Country: "USA",
	}
	parsed, err := parseLocationMemo([]byte(`geo:{"city":"Minneapolis","region":"MN","country":"USA"}`))
	assert.NoError(t, err)
	assert.Equal(t, expected, parsed)

	// errors
	_, err = parseLocationMemo([]byte(`geo:{"city":"Minneapolis","region":"MN","country":"USA}`))
	assert.Error(t, err)
	_, err = parseLocationMemo([]byte(`{"city":"Minneapolis","region":"MN","country":"USA"}`))
	assert.Error(t, err)
}
