package indexer

import (
	"errors"
	"strconv"
	"testing"

	"bridgerton.audius.co/database"
	"github.com/gagliardetto/solana-go"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/test-go/testify/assert"
	"go.uber.org/zap"
)

func TestUnprocessedTransactions(t *testing.T) {
	ctx := t.Context()
	pool := database.CreateTestDatabase(t, "test_solana_indexer")
	defer pool.Close()

	// Insert a test unprocessed transaction
	signature := "test_signature"
	errorMessage := "test error message"
	err := insertUnprocessedTransaction(ctx, pool, signature, errorMessage)
	require.NoError(t, err)

	// Verify the transaction was inserted
	res, err := getUnprocessedTransactions(ctx, pool, 10, 0)
	require.NoError(t, err)
	assert.Len(t, res, 1)
	assert.Equal(t, signature, res[0])

	// Delete the unprocessed transaction
	err = deleteUnprocessedTransaction(ctx, pool, signature)
	require.NoError(t, err)

	// Verify the transaction was deleted
	res, err = getUnprocessedTransactions(ctx, pool, 10, 0)
	require.NoError(t, err)
	assert.Len(t, res, 0)
}

func TestRetryUnprocessedTransactions(t *testing.T) {
	ctx := t.Context()
	pool := database.CreateTestDatabase(t, "test_solana_indexer")
	defer pool.Close()

	unprocessedTransactionsCount := 543
	processor := &mockProcessor{}

	var failingSigBytes [64]byte
	copy(failingSigBytes[:], []byte("test_signature_73"))
	failingSig := solana.SignatureFromBytes(failingSigBytes[:])

	// Mock the processor to fail on a specific signature
	processor.On("ProcessSignature", ctx, mock.Anything, failingSig, mock.Anything).
		Return(errors.New("fake failure")).Times(1)

	// Everything else should succeed
	processor.On("ProcessSignature", ctx, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(nil).Times(unprocessedTransactionsCount - 1)

	s := &SolanaIndexer{
		processor: processor,
		pool:      pool,
		logger:    zap.NewNop(),
	}

	for i := range unprocessedTransactionsCount {
		var sigBytes [64]byte
		copy(sigBytes[:], []byte("test_signature_"+strconv.FormatInt(int64(i), 10)))
		signature := solana.SignatureFromBytes(sigBytes[:])
		insertUnprocessedTransaction(ctx, pool, signature.String(), "test error message")
	}

	err := s.RetryUnprocessedTransactions(ctx)
	require.NoError(t, err)
	processor.AssertNumberOfCalls(t, "ProcessSignature", unprocessedTransactionsCount)

	// Verify all transactions but #73 were processed
	unprocessedTxs, err := getUnprocessedTransactions(ctx, pool, 100, 0)
	require.NoError(t, err)
	assert.Len(t, unprocessedTxs, 1, "expected a single unprocessed transaction after retry")
	assert.Equal(t, failingSig.String(), unprocessedTxs[0], "expected the failing transaction to remain unprocessed")
}
