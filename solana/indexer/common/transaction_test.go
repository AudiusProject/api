package common

import (
	"context"
	"testing"

	"api.audius.co/solana/indexer/fake_rpc_client"
	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
	"github.com/maypok86/otter"
	"github.com/test-go/testify/assert"
)

func TestFetchTransactionWithCache_CacheHit(t *testing.T) {
	cache, err := otter.MustBuilder[solana.Signature, *rpc.GetTransactionResult](10).
		Build()
	assert.NoError(t, err, "failed to create cache")
	sig := solana.Signature{}
	expected := &rpc.GetTransactionResult{}
	cache.Set(sig, expected)

	result, err := FetchTransactionWithCache(context.Background(), &cache, nil, sig)
	assert.NoError(t, err)
	assert.Equal(t, expected, result)
}

func TestFetchTransactionWithCache_CacheMiss(t *testing.T) {
	cache, err := otter.MustBuilder[solana.Signature, *rpc.GetTransactionResult](10).
		Build()
	assert.NoError(t, err, "failed to create cache")
	sig := solana.Signature{}
	expected := &rpc.GetTransactionResult{}
	fakeRpc := &fake_rpc_client.FakeRpcClient{
		GetTransactionFunc: func(ctx context.Context, signature solana.Signature, opts *rpc.GetTransactionOpts) (*rpc.GetTransactionResult, error) {
			return expected, nil
		},
	}
	result, err := FetchTransactionWithCache(context.Background(), &cache, fakeRpc, sig)
	assert.NoError(t, err)
	assert.Equal(t, expected, result)
}

func TestResolveLookupTables(t *testing.T) {
	tx := &solana.Transaction{
		Message: solana.Message{
			AddressTableLookups: []solana.MessageAddressTableLookup{
				{
					AccountKey:      solana.PublicKey{},
					WritableIndexes: []uint8{0},
					ReadonlyIndexes: []uint8{1},
				},
			},
		},
	}
	meta := &rpc.TransactionMeta{
		LoadedAddresses: rpc.LoadedAddresses{
			Writable: []solana.PublicKey{{1}},
			ReadOnly: []solana.PublicKey{{2}},
		},
	}
	ResolveLookupTables(context.Background(), nil, tx, meta)
	addressTables := tx.Message.GetAddressTables()
	assert.Equal(t, solana.PublicKey{1}, addressTables[solana.PublicKey{}][0])
	assert.Equal(t, solana.PublicKey{2}, addressTables[solana.PublicKey{}][1])
}
