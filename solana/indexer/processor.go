package indexer

import (
	"context"
	"fmt"
	"time"

	"bridgerton.audius.co/config"
	"bridgerton.audius.co/solana/spl/programs/claimable_tokens"
	"bridgerton.audius.co/solana/spl/programs/payment_router"
	"bridgerton.audius.co/solana/spl/programs/reward_manager"
	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
	"go.uber.org/zap"
)

type Processor interface {
	ProcessSignature(ctx context.Context, slot uint64, txSig solana.Signature, logger *zap.Logger) error
	ProcessTransaction(
		ctx context.Context,
		slot uint64,
		meta *rpc.TransactionMeta,
		tx *solana.Transaction,
		blockTime time.Time,
		logger *zap.Logger,
	) error
}

type DefaultProcessor struct {
	rpcClient   RpcClient
	pool        DbPool
	config      config.Config
	mintsFilter *[]string
}

func (p *DefaultProcessor) ProcessSignature(ctx context.Context, slot uint64, txSig solana.Signature, logger *zap.Logger) error {
	txRes, err := p.rpcClient.GetTransaction(
		ctx,
		txSig,
		&rpc.GetTransactionOpts{
			Commitment:                     rpc.CommitmentConfirmed,
			MaxSupportedTransactionVersion: &rpc.MaxSupportedTransactionVersion0,
		},
	)
	if err != nil {
		return fmt.Errorf("failed to get transaction: %w", err)
	}

	tx, err := txRes.Transaction.GetTransaction()
	if err != nil {
		return fmt.Errorf("failed to decode transaction: %w", err)
	}

	err = p.ProcessTransaction(ctx, slot, txRes.Meta, tx, txRes.BlockTime.Time(), logger)
	if err != nil {
		return fmt.Errorf("failed to process transaction: %w", err)
	}
	return nil
}

func (p *DefaultProcessor) ProcessTransaction(
	ctx context.Context,
	slot uint64,
	meta *rpc.TransactionMeta,
	tx *solana.Transaction,
	blockTime time.Time,
	logger *zap.Logger,
) error {
	sqlTx, err := p.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin sql transaction: %w", err)
	}
	defer sqlTx.Rollback(ctx)

	if tx == nil {
		return fmt.Errorf("no transaction to process")
	}
	if meta == nil {
		return fmt.Errorf("missing tx meta")
	}
	if logger == nil {
		logger = zap.NewNop()
	}
	txLogger := logger.With(
		zap.String("signature", tx.Signatures[0].String()),
	)

	// Resolve address lookup tables
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

	signature := tx.Signatures[0].String()

	err = processBalanceChanges(ctx, p.pool, slot, meta, tx, blockTime, txLogger)
	if err != nil {
		return fmt.Errorf("failed to process balance changes: %w", err)
	}

	for instructionIndex, instruction := range tx.Message.Instructions {
		programId := tx.Message.AccountKeys[instruction.ProgramIDIndex]
		instLogger := txLogger.With(
			zap.String("programId", programId.String()),
			zap.Int("instructionIndex", instructionIndex),
		)
		switch programId {
		case claimable_tokens.ProgramID:
			{
				err := processClaimableTokensInstruction(ctx, p.pool, slot, tx, instructionIndex, instruction, signature, instLogger)
				if err != nil {
					return fmt.Errorf("error processing claimable_tokens instruction %d: %w", instructionIndex, err)
				}
			}
		case reward_manager.ProgramID:
			{
				err := processRewardManagerInstruction(ctx, p.pool, slot, tx, instructionIndex, instruction, signature, instLogger)
				if err != nil {
					return fmt.Errorf("error processing reward_manager instruction %d: %w", instructionIndex, err)
				}
			}
		case payment_router.ProgramID:
			{
				err := processPaymentRouterInstruction(ctx, p.pool, slot, tx, instructionIndex, instruction, signature, blockTime, p.config, instLogger)
				if err != nil {
					return fmt.Errorf("error processing payment_router instruction %d: %w", instructionIndex, err)
				}
			}
		}
	}

	err = sqlTx.Commit(ctx)
	if err != nil {
		return fmt.Errorf("failed to commit sql transaction: %w", err)
	}
	return nil
}
