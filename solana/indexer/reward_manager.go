package indexer

import (
	"context"
	"fmt"
	"strings"

	"bridgerton.audius.co/database"
	"bridgerton.audius.co/solana/spl/programs/reward_manager"
	"github.com/gagliardetto/solana-go"
	"github.com/jackc/pgx/v5"
	"go.uber.org/zap"
)

func processRewardManagerInstruction(
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
		return fmt.Errorf("error resolving instruction accounts: %w", err)
	}
	inst, err := reward_manager.DecodeInstruction(accounts, []byte(instruction.Data))
	if err != nil {
		return fmt.Errorf("error decoding reward_manager instruction: %w", err)
	}
	switch inst.TypeID.Uint8() {
	case reward_manager.Instruction_EvaluateAttestations:
		if claimInst, ok := inst.Impl.(*reward_manager.EvaluateAttestation); ok {
			disbursementIdParts := strings.Split(claimInst.DisbursementId, ":")
			err := insertRewardDisbursement(ctx, db, rewardDisbursementsRow{
				signature:        signature,
				instructionIndex: instructionIndex,
				amount:           claimInst.Amount,
				slot:             slot,
				userBank:         claimInst.DestinationUserBankAccount().PublicKey.String(),
				challengeId:      disbursementIdParts[0],
				specifier:        strings.Join(disbursementIdParts[1:], ":"),
			})
			if err != nil {
				return fmt.Errorf("failed to insert reward disbursement at instruction: %w", err)
			}
			instLogger.Info("reward_manager evaluateAttestations",
				zap.String("ethAddress", claimInst.RecipientEthAddress.String()),
				zap.String("userBank", claimInst.DestinationUserBankAccount().PublicKey.String()),
				zap.Uint64("amount", claimInst.Amount),
				zap.String("disbursementId", claimInst.DisbursementId),
			)
		}
	}
	return nil
}

type rewardDisbursementsRow struct {
	signature        string
	instructionIndex int
	amount           uint64
	slot             uint64
	userBank         string
	challengeId      string
	specifier        string
}

func insertRewardDisbursement(ctx context.Context, db database.DBTX, row rewardDisbursementsRow) error {
	sql := `
		INSERT INTO sol_reward_disbursements
			(signature, instruction_index, amount, slot, user_bank, challenge_id, specifier)
		VALUES
			(@signature, @instructionIndex, @amount, @slot, @userBank, @challengeId, @specifier)
		ON CONFLICT DO NOTHING
	;`
	_, err := db.Exec(ctx, sql, pgx.NamedArgs{
		"signature":        row.signature,
		"instructionIndex": row.instructionIndex,
		"amount":           row.amount,
		"slot":             row.slot,
		"userBank":         row.userBank,
		"challengeId":      row.challengeId,
		"specifier":        row.specifier,
	})
	return err
}
