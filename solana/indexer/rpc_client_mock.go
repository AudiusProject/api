package indexer

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
)

// RpcClientFake allows tests to specify responses for each method.
type RpcClientFake struct {
	GetBlockWithOptsFunc                func(ctx context.Context, slot uint64, opts *rpc.GetBlockOpts) (*rpc.GetBlockResult, error)
	GetSlotFunc                         func(ctx context.Context, commitment rpc.CommitmentType) (uint64, error)
	GetSignaturesForAddressWithOptsFunc func(ctx context.Context, address solana.PublicKey, opts *rpc.GetSignaturesForAddressOpts) ([]*rpc.TransactionSignature, error)
	GetTransactionFunc                  func(ctx context.Context, sig solana.Signature, opts *rpc.GetTransactionOpts) (*rpc.GetTransactionResult, error)
}

func (m *RpcClientFake) GetBlockWithOpts(ctx context.Context, slot uint64, opts *rpc.GetBlockOpts) (*rpc.GetBlockResult, error) {
	if m.GetBlockWithOptsFunc != nil {
		return m.GetBlockWithOptsFunc(ctx, slot, opts)
	}
	return nil, nil
}

func (m *RpcClientFake) GetSlot(ctx context.Context, commitment rpc.CommitmentType) (uint64, error) {
	if m.GetSlotFunc != nil {
		return m.GetSlotFunc(ctx, commitment)
	}
	return 0, nil
}

func (m *RpcClientFake) GetSignaturesForAddressWithOpts(ctx context.Context, address solana.PublicKey, opts *rpc.GetSignaturesForAddressOpts) ([]*rpc.TransactionSignature, error) {
	if m.GetSignaturesForAddressWithOptsFunc != nil {
		return m.GetSignaturesForAddressWithOptsFunc(ctx, address, opts)
	}
	return nil, nil
}

func (m *RpcClientFake) GetTransaction(ctx context.Context, sig solana.Signature, opts *rpc.GetTransactionOpts) (*rpc.GetTransactionResult, error) {
	if m.GetTransactionFunc != nil {
		return m.GetTransactionFunc(ctx, sig, opts)
	}
	return nil, nil
}

func zipTransactionResultsAndTransactions(
	transactionResults []*rpc.GetTransactionResult,
	transactions []solana.Transaction,
) ([]*rpc.GetTransactionResult, error) {
	if len(transactionResults) > len(transactions) {
		return nil, errors.New("transaction results and transactions length mismatch")
	}
	for i := range transactionResults {
		txJson, err := json.Marshal(transactions[i])
		if err != nil {
			return nil, err
		}
		err = json.Unmarshal(txJson, &transactionResults[i].Transaction)
		if err != nil {
			return nil, err
		}
	}
	return transactionResults, nil
}

func NewRpcClientFakeFromTransactions(transactionResults []*rpc.GetTransactionResult) *RpcClientFake {
	return &RpcClientFake{
		GetSignaturesForAddressWithOptsFunc: func(ctx context.Context, address solana.PublicKey, opts *rpc.GetSignaturesForAddressOpts) ([]*rpc.TransactionSignature, error) {
			result := make([]*rpc.TransactionSignature, 0)
			startIndex := -1

			for i, txRes := range transactionResults {
				tx, err := txRes.Transaction.GetTransaction()
				if err != nil {
					return nil, err
				}
				if tx.Signatures[0] == opts.Before {
					startIndex = i
				}
			}
			for i, txRes := range transactionResults {
				tx, err := txRes.Transaction.GetTransaction()
				if err != nil {
					return nil, err
				}
				if tx.Signatures[0] == opts.Until {
					break
				}
				if i <= startIndex {
					continue
				}
				for _, acc := range tx.Message.AccountKeys {
					if acc.Equals(address) {
						result = append(result, &rpc.TransactionSignature{
							Signature: tx.Signatures[0],
							Slot:      txRes.Slot,
							Err:       txRes.Meta.Err,
							BlockTime: txRes.BlockTime,
						})
						break
					}
				}
				// hardcode a limit of 2 so that we can test pagination
				// not doing 1 because zero signatures need to be skipped w/o setting the new pointer
				if len(result) >= 2 {
					break
				}
			}
			return result, nil
		},
		GetBlockWithOptsFunc: func(ctx context.Context, slot uint64, opts *rpc.GetBlockOpts) (*rpc.GetBlockResult, error) {
			result := &rpc.GetBlockResult{}
			for _, txRes := range transactionResults {
				tx, err := txRes.Transaction.GetTransaction()
				if err != nil {
					return nil, err
				}
				if txRes.Slot == slot {
					result.Signatures = append(result.Signatures, tx.Signatures[0])
				}
			}
			return result, nil
		},
		GetTransactionFunc: func(ctx context.Context, sig solana.Signature, opts *rpc.GetTransactionOpts) (*rpc.GetTransactionResult, error) {
			for _, txRes := range transactionResults {
				tx, err := txRes.Transaction.GetTransaction()
				if err != nil {
					return nil, err
				}
				if tx.Signatures[0] == sig {
					return txRes, nil
				}
			}
			return nil, rpc.ErrNotFound
		},
	}
}
