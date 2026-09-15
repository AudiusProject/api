package common

import (
	"context"
	"testing"

	"api.audius.co/solana/indexer/fake_rpc_client"
	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Solana Transaction v1 activated on mainnet in epoch 1035. RPC providers
// reject getTransaction for a v1 transaction unless the client declares it can
// handle v1, so the indexer must always request maxSupportedTransactionVersion=1.
func TestFetchTransactionWithCacheRequestsTransactionVersion1(t *testing.T) {
	var gotOpts *rpc.GetTransactionOpts
	fake := &fake_rpc_client.FakeRpcClient{
		GetTransactionFunc: func(ctx context.Context, sig solana.Signature, opts *rpc.GetTransactionOpts) (*rpc.GetTransactionResult, error) {
			gotOpts = opts
			return &rpc.GetTransactionResult{}, nil
		},
	}

	_, err := FetchTransactionWithCache(context.Background(), nil, fake, solana.Signature{})
	require.NoError(t, err)
	require.NotNil(t, gotOpts)
	require.NotNil(t, gotOpts.MaxSupportedTransactionVersion)
	assert.Equal(t, uint64(1), *gotOpts.MaxSupportedTransactionVersion)
}
