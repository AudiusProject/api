package indexer

import (
	"context"
	"testing"
	"time"

	"bridgerton.audius.co/solana/indexer/fake_rpc_client"
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

type MockProcessor struct {
	mock.Mock
}

func (m *MockProcessor) ProcessSignature(ctx context.Context, slot uint64, txSig solana.Signature, logger *zap.Logger) error {
	args := m.Called(ctx, slot, txSig, logger)
	return args.Error(0)
}
func (m *MockProcessor) ProcessTransaction(
	ctx context.Context,
	slot uint64,
	meta *rpc.TransactionMeta,
	tx *solana.Transaction,
	blockTime time.Time,
	logger *zap.Logger,
) error {
	args := m.Called(ctx, slot, meta, tx, blockTime, logger)
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

	mockTransactionResponses, err := fake_rpc_client.ZipTransactionResultsAndTransactions(mockTransactionResponses, mockTransactions)
	require.NoError(t, err, "failed to zip transaction results and transactions")
	rpcFake := fake_rpc_client.NewWithTransactions(mockTransactionResponses)

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

	mockTransactionResponses, err := fake_rpc_client.ZipTransactionResultsAndTransactions(mockTransactionResponses, mockTransactions)
	require.NoError(t, err, "failed to zip transaction results and transactions")
	rpcFake := fake_rpc_client.NewWithTransactions(mockTransactionResponses)

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
