package indexer

import (
	"context"
	"testing"
	"time"

	"bridgerton.audius.co/database"
	"bridgerton.audius.co/solana/spl/programs/claimable_tokens"
	"bridgerton.audius.co/solana/spl/programs/payment_router"
	"bridgerton.audius.co/solana/spl/programs/reward_manager"
	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
	"github.com/pashagolub/pgxmock/v4"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/test-go/testify/assert"
	"go.uber.org/zap"
)

// poolMock.ExpectQuery(`SELECT EXISTS`).
// 	WithArgs(pgxmock.AnyArg()).
// 	WillReturnRows(
// 		pgxmock.NewRows([]string{"exists"}).
// 			AddRow(false),
// 	)

// poolMock.ExpectBegin()

// poolMock.MatchExpectationsInOrder(false)

// poolMock.ExpectExec(`INSERT INTO sol_claimable_account_transfers`).
// 	WithArgs(
// 		pgxmock.AnyArg(),
// 		pgxmock.AnyArg(),
// 		pgxmock.AnyArg(),
// 		pgxmock.AnyArg(),
// 		pgxmock.AnyArg(),
// 		pgxmock.AnyArg(),
// 		pgxmock.AnyArg(),
// 	).
// 	WillReturnResult(pgxmock.NewResult("INSERT", 1))

// poolMock.ExpectExec(`INSERT INTO sol_payments`).
// 	WithArgs(
// 		pgxmock.AnyArg(),
// 		pgxmock.AnyArg(),
// 		pgxmock.AnyArg(),
// 		pgxmock.AnyArg(),
// 		pgxmock.AnyArg(),
// 		pgxmock.AnyArg(),
// 	).
// 	WillReturnResult(pgxmock.NewResult("INSERT", 1))

// poolMock.ExpectExec(`INSERT INTO sol_payments`).
// 	WithArgs(
// 		pgxmock.AnyArg(),
// 		pgxmock.AnyArg(),
// 		pgxmock.AnyArg(),
// 		pgxmock.AnyArg(),
// 		pgxmock.AnyArg(),
// 		pgxmock.AnyArg(),
// 	).
// 	WillReturnResult(pgxmock.NewResult("INSERT", 1))

// poolMock.ExpectExec(`INSERT INTO sol_purchases`).
// 	WithArgs(
// 		pgxmock.AnyArg(),
// 		pgxmock.AnyArg(),
// 		pgxmock.AnyArg(),
// 		pgxmock.AnyArg(),
// 		pgxmock.AnyArg(),
// 		pgxmock.AnyArg(),
// 		pgxmock.AnyArg(),
// 		pgxmock.AnyArg(),
// 		pgxmock.AnyArg(),
// 		pgxmock.AnyArg(),
// 		pgxmock.AnyArg(),
// 		pgxmock.AnyArg(),
// 		pgxmock.AnyArg(),
// 		pgxmock.AnyArg(),
// 	).
// 	WillReturnResult(pgxmock.NewResult("INSERT", 1))

// poolMock.ExpectCommit()

// // Purchase transaction
// // TODO: replace with a constructed transaction
// tx1, err := solana.TransactionFromBase64("AlVaYFHEFi1JvLs9gFKd7377my3f1qx957Hy3NSA7hHLvMH+umDKJwRriRux9hpwJjOQ0UDzpJM+RIreyGeA7wLO+aGNdAEsN6CdHe6ZlbIGXPmMAudKBn5/KIrNuMokozLTRsr+poOErwV9AZ4/2n+qQFqvHkfbX/xJ9r/T1qEKgAIABgsMQaUZPJhXqbeqCDdNQLlTjU/ztt7ZF2Jj8K6cEewWgu7KrSpGSx5qudroeqWsEXPwd0LKxGsQghlBXl7bqSNeHZekh1Kw56iSAWlRRw8qWzl0Ic0oB9neQ9DgJmJlFYlhM0Gn3spQ9Fee8x2/dhn6ij/lg+GJJk70ksJoWr0WPu1D4IUDLrA0TJHYtZIFVu4kxJjvdCUXCiN7nQWd0rLdBUpTUPhdyILWFKVWcniKKW3fHqur0KYGeIhJMvTu9qAG3fbh12Whk9nL4UbO63msHLSF7V9bN5E6jPWFfv8Aqcb6evO+2606PWXzaqvJdDGxu+TC0vbg5HymAgNFL11hDDC4ZKXZ8xNtdKlw5YBMugbzGaZaVUM7bXvwET7Z401s4UD/vcjS+CXoLDxmXaspL5SyJbaD7kBrJ/jiuJwQvgMGRm/lIRcy/+ytunLDm+e8jOW7xfcSayxDmzpAAAAAApDl0oOd40M/JTb5AAfoGADyMUJsAWsfXTf23HAdQaEFBQEBElJlY292ZXIgV2l0aGRyYXdhbAYEAgcDAQoMgJaYAAAAAAAGCAQDCQYEHeUXy5d6460q/gEAAACAlpgAAAAAAICWmAAAAAAACgAJA/BJAgAAAAAACgAFAqWFAAAA")
// require.NoError(t, err, "failed to parse transaction from base64")
// mockTransactions = append(mockTransactions, *tx1)

// // Another purchase
// tx2, err := solana.TransactionFromBase64("AVAxgMpKXolCpyP2+ew1MGZTA4sbraukKJeANkY/9aI1pAp9iCJ1EEmySwXehgM9rDUEsDxNT8Ry6b6uU+9UrgeAAQALEQST1w+Y6aLadF4Kw0n4lxaisLs2ff4aoRRSIEo+y/8ERNcqk0sXV+/t+ZifMcVymZFrHdXfqxdiT9S2ZZcyw+thM0Gn3spQ9Fee8x2/dhn6ij/lg+GJJk70ksJoWr0WPuWYG6kjVkyjykWKvcYduvEY6ZgwQepe7gk8YyylnPBi+hqbtDEvtJZzhv7ZJPk5EUF4Sn8j+mxkipDRm2Sue0JmywqVd7SHXkdj4ki3Nffd0GmHGvL4gB4zHYCiU+DAOgTG/CDwUMzwVYTXIRyfjPWewUeFuxZqHigw6BIgAAAAzy7yr4hXzw3AntZG7Z/mCKK1LDc3DCjviNxJUrxiB83QQBm+2natZtdlxJC/tn3eX37kUUln29E+oByMpxOuDQan1RcZLFxRIYzJTD1K8X9Y2u4Im6H9ROPb2YoAAAAABqfVFxh70WY12tQEVf3CwMEkxo8hVnWl27rLXwgAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAbd9uHXZaGT2cvhRs7reawctIXtX1s3kTqM9YV+/wCpDDC4ZKXZ8xNtdKlw5YBMugbzGaZaVUM7bXvwET7Z401s4UD/vcjS+CXoLDxmXaspL5SyJbaD7kBrJ/jiuJwQvgVKU1qZKSEGTSTocWDaOHx8NbXdvJK7geQfqEBBBUSNAwZGb+UhFzL/7K26csOb57yM5bvF9xJrLEObOkAAAACoXTJJDzQyQ64bXfSmIt6nQwtKNockFTdbrQNtjgoitwcGAJEBASAAAAwAAGEAMAAA5IS1JCCiMb9FjkxbHjTQF68QpUYvM+VYSUw+VxYrlmuX2PGdhEo5hfkDWT+RrGrDOfW/OV+IFfxfmU/nnQwzhARrEenkeAsgkPxAx6ebcxqJC0qIAGEzQafeylD0V57zHb92GfqKP+WD4YkmTvSSwmhavRY+QEIPAAAAAAABAAAAAAAAAAcJAAECAwgJCgsMFQHkhLUkIKIxv0WOTFseNNAXrxClRg0FAg4MBAUl5RfLl3rjrSr+AgAAAKC7DQAAAAAAoIYBAAAAAABAQg8AAAAAAA8AK3RyYWNrOjE0NTg4OTA2NDc6MTAxNTc2MDU1Ojg3NTM3ODg1NzpzdHJlYW0PAEVnZW86eyJjaXR5IjoiTmV3YXJrIiwicmVnaW9uIjoiTmV3IEplcnNleSIsImNvdW50cnkiOiJVbml0ZWQgU3RhdGVzIn0QAAkD8EkCAAAAAAAQAAUCYhUCAAA=")
// require.NoError(t, err, "failed to parse transaction from base64")
// mockTransactions = append(mockTransactions, *tx2)

// // Claimable tokens transfer transaction
// tx3, err := solana.TransactionFromBase64("Ac/jKV5ZFpXYkJWK899i/k/2qW/jnnv9+IDXUUb+dSfQCh5Ne7Hf80knQtIYctctzkAraaDo1/eV+xSJ9wrhdg+AAQAIDAST1w+Y6aLadF4Kw0n4lxaisLs2ff4aoRRSIEo+y/8EaQE23Y3gULwTbYJc7D0jseZBBcYrMI3A0LrqNi1sy9PzVT8e1pJsiVUkgdG2Ac2T7bHjs96ksRTtQ0DdnI5oEqrVy9+2m0n6H2ec7vWF2B3CHy0VG2D1VT4nYwCyc+QyBMb8IPBQzPBVhNchHJ+M9Z7BR4W7FmoeKDDoEiAAAADPLvKviFfPDcCe1kbtn+YIorUsNzcMKO+I3ElSvGIHzUPP8mVBQWg25I3w6C+/HfRvUl5fLKmKECTCRggokeZlBqfVFxksXFEhjMlMPUrxf1ja7gibof1E49vZigAAAAAGp9UXGHvRZjXa1ARV/cLAwSTGjyFWdaXbustfCAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAABt324ddloZPZy+FGzut5rBy0he1fWzeROoz1hX7/AKkDBkZv5SEXMv/srbpyw5vnvIzlu8X3EmssQ5s6QAAAAIgb+vrdQCfYuxCdysDVa6AqBN70pikZk8mhk/X8+ImVBAQAkQEBIAAADAAAYQAwAAArOevK2DQIWPGYEdpMtu0t8G7bLVgsgkWiCp6D2RIg9gqGtZYi+Z6tcB3KMqBcxiIJbD/aDeDZOZGMq1TsWdAujCjBXBG40NeNdMnEkosDLvJ5uakB81U/HtaSbIlVJIHRtgHNk+2x47PepLEU7UNA3ZyOaBIAq5BBAAAAAAAAAAAAAAAABQkAAQIDBgcICQoVASs568rYNAhY8ZgR2ky27S3wbtstCwAJA/BJAgAAAAAACwAFAm61AAAA")
// require.NoError(t, err, "failed to parse transaction from base64")
// mockTransactions = append(mockTransactions, *tx3)

type MockProcessor struct {
	mock.Mock
}

func (m *MockProcessor) ProcessSignature(ctx context.Context, slot uint64, txSig solana.Signature, logger *zap.Logger) error {
	args := m.Called(ctx, slot, txSig, logger)
	return args.Error(0)
}
func (m *MockProcessor) ProcessTransaction(
	ctx context.Context,
	db database.DBTX,
	slot uint64,
	meta *rpc.TransactionMeta,
	tx *solana.Transaction,
	blockTime time.Time,
	logger *zap.Logger,
) error {
	args := m.Called(ctx, db, slot, meta, tx, blockTime, logger)
	return args.Error(0)
}

// Tests that backfilling will fill the gap between the indexed transactions
// of the given slots, and will ignore error, existing, or zero signatures.
func TestBackfillContinue(t *testing.T) {
	mockTransactions := []solana.Transaction{
		{
			Signatures: []solana.Signature{{0x01, 0x02, 0x03}},
			Message: solana.Message{
				AccountKeys: []solana.PublicKey{
					claimable_tokens.ProgramID,
				},
			},
		},
		{
			Signatures: []solana.Signature{{0x04, 0x05, 0x06}},
			Message: solana.Message{
				AccountKeys: []solana.PublicKey{
					claimable_tokens.ProgramID,
				},
			},
		},
		// Ignored (transaction error)
		{
			Signatures: []solana.Signature{{0x05, 0x05, 0x06}},
			Message: solana.Message{
				AccountKeys: []solana.PublicKey{
					claimable_tokens.ProgramID,
				},
			},
		},
		// Ignored (zero signature)
		{
			Signatures: []solana.Signature{{}},
			Message: solana.Message{
				AccountKeys: []solana.PublicKey{
					claimable_tokens.ProgramID,
				},
			},
		},
		// Ignored (existing signature)
		{
			Signatures: []solana.Signature{{0x06, 0x05, 0x06}}, // not actually a dupe, handled by pgxmock
			Message: solana.Message{
				AccountKeys: []solana.PublicKey{
					claimable_tokens.ProgramID,
				},
			},
		},
		{
			Signatures: []solana.Signature{{0x07, 0x08, 0x09}},
			Message: solana.Message{
				AccountKeys: []solana.PublicKey{
					claimable_tokens.ProgramID,
				},
			},
		},
		// Ignored (past slot limit)
		{
			Signatures: []solana.Signature{{0x07, 0x07, 0x09}},
			Message: solana.Message{
				AccountKeys: []solana.PublicKey{
					claimable_tokens.ProgramID,
				},
			},
		},
	}
	now := solana.UnixTimeSeconds(time.Now().Unix())
	mockTransactionResponses := []*rpc.GetTransactionResult{
		{
			Slot:      200,
			BlockTime: &now,
			Meta: &rpc.TransactionMeta{
				PreTokenBalances:  []rpc.TokenBalance{},
				PostTokenBalances: []rpc.TokenBalance{},
			},
		},
		{
			Slot:      199,
			BlockTime: &now,
			Meta: &rpc.TransactionMeta{
				PreTokenBalances:  []rpc.TokenBalance{},
				PostTokenBalances: []rpc.TokenBalance{},
			},
		},
		// Skips errors
		{
			Slot:      198,
			BlockTime: &now,
			Meta: &rpc.TransactionMeta{
				Err:               "transaction error",
				PreTokenBalances:  []rpc.TokenBalance{},
				PostTokenBalances: []rpc.TokenBalance{},
			},
		},
		// Skips zero signatures
		{
			Slot:      197,
			BlockTime: &now,
			Meta: &rpc.TransactionMeta{
				PreTokenBalances:  []rpc.TokenBalance{},
				PostTokenBalances: []rpc.TokenBalance{},
			},
		},
		// Skips existing signature
		{
			Slot:      197,
			BlockTime: &now,
			Meta: &rpc.TransactionMeta{
				PreTokenBalances:  []rpc.TokenBalance{},
				PostTokenBalances: []rpc.TokenBalance{},
			},
		},
		{
			Slot:      100,
			BlockTime: &now,
			Meta: &rpc.TransactionMeta{
				PreTokenBalances:  []rpc.TokenBalance{},
				PostTokenBalances: []rpc.TokenBalance{},
			},
		},
		{
			Slot:      80,
			BlockTime: &now,
			Meta: &rpc.TransactionMeta{
				PreTokenBalances:  []rpc.TokenBalance{},
				PostTokenBalances: []rpc.TokenBalance{},
			},
		},
	}

	mockTransactionResponses, err := zipTransactionResultsAndTransactions(mockTransactionResponses, mockTransactions)
	require.NoError(t, err, "failed to zip transaction results and transactions")
	rpcFake := NewRpcClientFakeFromTransactions(mockTransactionResponses)

	poolMock, err := pgxmock.NewPool()
	require.NoError(t, err, "failed to create mock database pool")
	defer poolMock.Close()

	poolMock.MatchExpectationsInOrder(false)

	// Return some random signatures for the slot range so
	// that the backfill processes all the rpc transactions.
	rangeSigs := solana.Signature{0x08}.String()
	poolMock.ExpectQuery(`SELECT`).
		WithArgs(uint64(100), uint64(200)).
		WillReturnRows(
			pgxmock.NewRows([]string{"before", "until"}).
				AddRow(&rangeSigs, &rangeSigs),
		)

	poolMock.ExpectQuery(`SELECT EXISTS`).
		WithArgs(mockTransactions[0].Signatures[0]).
		WillReturnRows(
			pgxmock.NewRows([]string{"exists"}).
				AddRow(false),
		)
	poolMock.ExpectQuery(`SELECT EXISTS`).
		WithArgs(mockTransactions[1].Signatures[0]).
		WillReturnRows(
			pgxmock.NewRows([]string{"exists"}).
				AddRow(false),
		)
	poolMock.ExpectQuery(`SELECT EXISTS`).
		WithArgs(mockTransactions[4].Signatures[0]).
		WillReturnRows(
			pgxmock.NewRows([]string{"exists"}).
				AddRow(true), // EXISTS
		)
	poolMock.ExpectQuery(`SELECT EXISTS`).
		WithArgs(mockTransactions[5].Signatures[0]).
		WillReturnRows(
			pgxmock.NewRows([]string{"exists"}).
				AddRow(false),
		)

	processorMock := &MockProcessor{}
	processorMock.On("ProcessSignature", mock.Anything, mock.Anything, mockTransactions[0].Signatures[0], mock.Anything).
		Return(nil).Once()
	processorMock.On("ProcessSignature", mock.Anything, mock.Anything, mockTransactions[1].Signatures[0], mock.Anything).
		Return(nil).Once()
	processorMock.On("ProcessSignature", mock.Anything, mock.Anything, mockTransactions[5].Signatures[0], mock.Anything).
		Return(nil).Once()

	s := &SolanaIndexer{
		rpcClient: rpcFake,
		pool:      poolMock,
		processor: processorMock,
		logger:    zap.NewNop(),
	}

	err = s.Backfill(context.Background(), 100, 200)

	assert.NoError(t, err)
	assert.NoError(t, poolMock.ExpectationsWereMet())
	processorMock.AssertExpectations(t)
}

// Same as above test, except it fetches the bookends of the backfill using GetBlock
// instead of from the database. This ensures backfilling works even if there are no
// existing transactions in the database for the given range.
func TestBackfillFresh(t *testing.T) {
	mockTransactions := []solana.Transaction{
		{
			Signatures: []solana.Signature{{0x01, 0x02, 0x03}},
			Message: solana.Message{
				AccountKeys: []solana.PublicKey{
					claimable_tokens.ProgramID,
				},
			},
		},
		{
			Signatures: []solana.Signature{{0x04, 0x05, 0x06}},
			Message: solana.Message{
				AccountKeys: []solana.PublicKey{
					claimable_tokens.ProgramID,
					payment_router.ProgramID,
					reward_manager.ProgramID,
				},
			},
		},
		// Ignored (transaction error)
		{
			Signatures: []solana.Signature{{0x05, 0x05, 0x06}},
			Message: solana.Message{
				AccountKeys: []solana.PublicKey{
					claimable_tokens.ProgramID,
				},
			},
		},
		{
			// Ignored (zero signature)
			Signatures: []solana.Signature{{}},
			Message: solana.Message{
				AccountKeys: []solana.PublicKey{
					claimable_tokens.ProgramID,
				},
			},
		},
		{
			// Ignored (existing signature)
			Signatures: []solana.Signature{{0x06, 0x05, 0x06}}, // not actually a dupe, handled by pgxmock
			Message: solana.Message{
				AccountKeys: []solana.PublicKey{
					claimable_tokens.ProgramID,
				},
			},
		},
		{
			Signatures: []solana.Signature{{0x07, 0x08, 0x09}},
			Message: solana.Message{
				AccountKeys: []solana.PublicKey{
					claimable_tokens.ProgramID,
				},
			},
		},
		// Past slot limit
		{
			Signatures: []solana.Signature{{0x07, 0x07, 0x09}},
			Message: solana.Message{
				AccountKeys: []solana.PublicKey{
					claimable_tokens.ProgramID,
				},
			},
		},
	}
	now := solana.UnixTimeSeconds(time.Now().Unix())
	mockTransactionResponses := []*rpc.GetTransactionResult{
		{
			Slot:      200,
			BlockTime: &now,
			Meta: &rpc.TransactionMeta{
				PreTokenBalances:  []rpc.TokenBalance{},
				PostTokenBalances: []rpc.TokenBalance{},
			},
		},
		{
			Slot:      199,
			BlockTime: &now,
			Meta: &rpc.TransactionMeta{
				PreTokenBalances:  []rpc.TokenBalance{},
				PostTokenBalances: []rpc.TokenBalance{},
			},
		},
		{
			Slot:      100,
			BlockTime: &now,
			Meta: &rpc.TransactionMeta{
				PreTokenBalances:  []rpc.TokenBalance{},
				PostTokenBalances: []rpc.TokenBalance{},
			},
		},
		{
			Slot:      80,
			BlockTime: &now,
			Meta: &rpc.TransactionMeta{
				PreTokenBalances:  []rpc.TokenBalance{},
				PostTokenBalances: []rpc.TokenBalance{},
			},
		},
	}

	mockTransactionResponses, err := zipTransactionResultsAndTransactions(mockTransactionResponses, mockTransactions)
	require.NoError(t, err, "failed to zip transaction results and transactions")
	rpcFake := NewRpcClientFakeFromTransactions(mockTransactionResponses)

	poolMock, err := pgxmock.NewPool()
	require.NoError(t, err, "failed to create mock database pool")
	defer poolMock.Close()

	poolMock.MatchExpectationsInOrder(false)

	// No existing transactions around the slot range
	poolMock.ExpectQuery(`SELECT`).
		WithArgs(uint64(100), uint64(200)).
		WillReturnRows(
			pgxmock.NewRows([]string{"before", "until"}).
				AddRow(nil, nil),
		)

	// One call for each program
	poolMock.ExpectQuery(`SELECT EXISTS`).
		WithArgs(mockTransactions[1].Signatures[0]).
		WillReturnRows(
			pgxmock.NewRows([]string{"exists"}).
				AddRow(false),
		)
	poolMock.ExpectQuery(`SELECT EXISTS`).
		WithArgs(mockTransactions[1].Signatures[0]).
		WillReturnRows(
			pgxmock.NewRows([]string{"exists"}).
				AddRow(false),
		)
	poolMock.ExpectQuery(`SELECT EXISTS`).
		WithArgs(mockTransactions[1].Signatures[0]).
		WillReturnRows(
			pgxmock.NewRows([]string{"exists"}).
				AddRow(false),
		)

	processorMock := &MockProcessor{}

	// Should get called once for each program
	processorMock.On("ProcessSignature", mock.Anything, mock.Anything, mockTransactions[1].Signatures[0], mock.Anything).
		Return(nil).Times(3)

	s := &SolanaIndexer{
		rpcClient: rpcFake,
		pool:      poolMock,
		processor: processorMock,
		logger:    zap.NewNop(),
	}

	err = s.Backfill(context.Background(), 100, 200)

	assert.NoError(t, err)
	assert.NoError(t, poolMock.ExpectationsWereMet())
	processorMock.AssertExpectations(t)
}
