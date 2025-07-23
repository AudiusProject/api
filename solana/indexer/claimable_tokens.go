package indexer

import (
	"context"
	"fmt"
	"strings"

	"bridgerton.audius.co/database"
	"bridgerton.audius.co/solana/spl/programs/claimable_tokens"
	"bridgerton.audius.co/solana/spl/programs/secp256k1"
	bin "github.com/gagliardetto/binary"
	"github.com/gagliardetto/solana-go"
	"github.com/jackc/pgx/v5"
	"go.uber.org/zap"
)

func processClaimableTokensInstruction(
	ctx context.Context,
	db database.DBTX,
	slot uint64,
	tx *solana.Transaction,
	instructionIndex int,
	instruction solana.CompiledInstruction,
	signature string,
	instLogger *zap.Logger,
) error {
	accounts, err := instruction.ResolveInstructionAccounts(&tx.Message)
	if err != nil {
		return fmt.Errorf("error resolving instruction accounts %d: %w", instructionIndex, err)
	}

	inst, err := claimable_tokens.DecodeInstruction(accounts, []byte(instruction.Data))
	if err != nil {
		return fmt.Errorf("error decoding claimable_tokens instruction %d: %w", instructionIndex, err)
	}
	switch inst.TypeID.Uint8() {
	case claimable_tokens.Instruction_CreateTokenAccount:
		{
			if createInst, ok := inst.Impl.(*claimable_tokens.CreateTokenAccount); ok {
				err := insertClaimableAccount(ctx, db, claimableAccountsRow{
					signature:        signature,
					instructionIndex: instructionIndex,
					slot:             slot,
					mint:             createInst.Mint().PublicKey.String(),
					ethereumAddress:  strings.ToLower(createInst.EthAddress.Hex()),
					account:          createInst.UserBank().PublicKey.String(),
				})
				if err != nil {
					return fmt.Errorf("failed to insert claimable tokens account at instruction %d: %w", instructionIndex, err)
				}
				instLogger.Debug("claimable_tokens createTokenAccount",
					zap.String("mint", createInst.Mint().PublicKey.String()),
					zap.String("userBank", createInst.UserBank().PublicKey.String()),
					zap.String("ethAddress", createInst.EthAddress.String()),
				)
			}
		}
	case claimable_tokens.Instruction_Transfer:
		{
			if transferInst, ok := inst.Impl.(*claimable_tokens.Transfer); ok {
				var signedData claimable_tokens.SignedTransferData
				// The signed Secp256k1Instruction must be directly before the transfer
				secpInstruction := tx.Message.Instructions[instructionIndex-1]
				accounts, err := secpInstruction.ResolveInstructionAccounts(&tx.Message)
				if err != nil {
					return fmt.Errorf("failed to resolve instruction accounts at instruction %d: %w", instructionIndex-1, err)
				}
				secpInstRaw, err := secp256k1.DecodeInstruction(accounts, secpInstruction.Data)
				if err != nil {
					return fmt.Errorf("failed to decode secp256k1 instruction %d: %w", instructionIndex-1, err)
				}
				if secpInst, ok := secpInstRaw.Impl.(*secp256k1.Secp256k1Instruction); ok {
					dec := bin.NewBinDecoder(secpInst.SignatureDatas[0].Message)
					err := dec.Decode(&signedData)
					if err != nil {
						return fmt.Errorf("failed to parse signed transfer data at instruction %d: %w", instructionIndex-1, err)
					}
				}
				err = insertClaimableAccountTransfer(ctx, db, claimableAccountTransfersRow{
					signature:        signature,
					instructionIndex: instructionIndex,
					amount:           signedData.Amount,
					slot:             slot,
					fromAccount:      transferInst.SenderUserBank().PublicKey.String(),
					toAccount:        transferInst.Destination().PublicKey.String(),
					senderEthAddress: strings.ToLower(transferInst.SenderEthAddress.Hex()),
				})
				if err != nil {
					return fmt.Errorf("failed to insert claimable tokens transfer at instruction %d: %w", instructionIndex, err)
				}
				instLogger.Info("claimable_tokens transfer",
					zap.String("ethAddress", transferInst.SenderEthAddress.String()),
					zap.String("userBank", transferInst.SenderUserBank().PublicKey.String()),
					zap.String("destination", transferInst.Destination().PublicKey.String()),
					zap.Uint64("amount", signedData.Amount),
				)
			}
		}
	}
	return nil
}

type claimableAccountsRow struct {
	signature        string
	instructionIndex int
	slot             uint64
	mint             string
	ethereumAddress  string
	account          string
}

func insertClaimableAccount(ctx context.Context, db database.DBTX, row claimableAccountsRow) error {
	sql := `
		INSERT INTO sol_claimable_accounts
			(signature, instruction_index, slot, mint, ethereum_address, account)
		VALUES
			(@signature, @instructionIndex, @slot, @mint, @ethereumAddress, @account)
		ON CONFLICT DO NOTHING
	;`
	_, err := db.Exec(ctx, sql, pgx.NamedArgs{
		"signature":        row.signature,
		"instructionIndex": row.instructionIndex,
		"slot":             row.slot,
		"mint":             row.mint,
		"ethereumAddress":  row.ethereumAddress,
		"account":          row.account,
	})
	return err
}

type claimableAccountTransfersRow struct {
	signature        string
	instructionIndex int
	amount           uint64
	slot             uint64
	fromAccount      string
	toAccount        string
	senderEthAddress string
}

func insertClaimableAccountTransfer(ctx context.Context, db database.DBTX, row claimableAccountTransfersRow) error {
	sql := `
		INSERT INTO sol_claimable_account_transfers
			(signature, instruction_index, amount, slot, from_account, to_account, sender_eth_address)
		VALUES
			(@signature, @instructionIndex, @amount, @slot, @fromAccount, @toAccount, @senderEthAddress)
		ON CONFLICT DO NOTHING
	;`
	_, err := db.Exec(ctx, sql, pgx.NamedArgs{
		"signature":        row.signature,
		"instructionIndex": row.instructionIndex,
		"amount":           row.amount,
		"slot":             row.slot,
		"fromAccount":      row.fromAccount,
		"toAccount":        row.toAccount,
		"senderEthAddress": row.senderEthAddress,
	})
	return err
}
