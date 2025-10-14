package common

import (
	"context"
	"fmt"
	"time"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
	"github.com/maypok86/otter"
)

// Gets a transaction from a cache or fetches it from the RPC. Handles retries.
func FetchTransactionWithCache(
	ctx context.Context,
	transactionCache *otter.Cache[solana.Signature,
		*rpc.GetTransactionResult],
	rpcClient RpcClient,
	signature solana.Signature,
) (*rpc.GetTransactionResult, error) {
	// Check if the transaction is in the cache
	if transactionCache != nil {
		if res, ok := transactionCache.Get(signature); ok {
			return res, nil
		}
	}

	// If the transaction is not in the cache, fetch it from the RPC
	res, err := WithRetriesResult(func() (*rpc.GetTransactionResult, error) {
		return rpcClient.GetTransaction(
			ctx,
			signature,
			&rpc.GetTransactionOpts{
				Commitment:                     rpc.CommitmentConfirmed,
				MaxSupportedTransactionVersion: &rpc.MaxSupportedTransactionVersion0,
			},
		)
	}, 5, 1*time.Second)
	if err != nil {
		return nil, fmt.Errorf("failed to get transaction: %w", err)
	}

	// Store the fetched transaction in the cache
	if transactionCache != nil {
		transactionCache.Set(signature, res)
	}

	return res, nil
}

// Resolves address lookup tables in the given transaction using the provided metadata.
func ResolveLookupTables(
	ctx context.Context,
	rpcClient RpcClient,
	tx *solana.Transaction,
	meta *rpc.TransactionMeta,
) *solana.Transaction {
	addressTables := make(map[solana.PublicKey]solana.PublicKeySlice)
	writablePos := 0
	readonlyPos := 0
	for _, lu := range tx.Message.AddressTableLookups {
		addresses := make(solana.PublicKeySlice, 256)
		for _, idx := range lu.WritableIndexes {
			addresses[idx] = meta.LoadedAddresses.Writable[writablePos]
			writablePos += 1
		}
		for _, idx := range lu.ReadonlyIndexes {
			addresses[idx] = meta.LoadedAddresses.ReadOnly[readonlyPos]
			readonlyPos += 1
		}
		addressTables[lu.AccountKey] = addresses
	}
	tx.Message.SetAddressTables(addressTables)
	return tx
}
