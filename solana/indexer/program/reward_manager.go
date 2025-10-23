package program

import (
	"context"
	"fmt"
	"strings"

	"api.audius.co/database"
	"api.audius.co/solana/spl/programs/reward_manager"
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
				signature:           signature,
				instructionIndex:    instructionIndex,
				amount:              claimInst.Amount,
				slot:                slot,
				userBank:            claimInst.DestinationUserBankAccount().PublicKey.String(),
				challengeId:         disbursementIdParts[0],
				specifier:           strings.Join(disbursementIdParts[1:], ":"),
				recipientEthAddress: strings.ToLower(claimInst.RecipientEthAddress.String()),
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
	case reward_manager.Instruction_Init:
		if initInst, ok := inst.Impl.(*reward_manager.Init); ok {
			err := insertRewardInit(ctx, db, rewardInitRow{
				signature:          signature,
				instructionIndex:   instructionIndex,
				slot:               slot,
				minVotes:           initInst.MinVotes,
				rewardManagerState: initInst.RewardManagerState().PublicKey.String(),
				tokenSource:        initInst.TokenSourceAccount().PublicKey.String(),
				mint:               initInst.Mint().PublicKey.String(),
				manager:            initInst.Manager().PublicKey.String(),
				authority:          initInst.Authority().PublicKey.String(),
			})
			if err != nil {
				return fmt.Errorf("failed to insert reward init at instruction: %w", err)
			}
			instLogger.Info("reward_manager init",
				zap.Uint8("minVotes", initInst.MinVotes),
				zap.String("rewardManagerState", initInst.RewardManagerState().PublicKey.String()),
				zap.String("tokenSource", initInst.TokenSourceAccount().PublicKey.String()),
				zap.String("mint", initInst.Mint().PublicKey.String()),
				zap.String("manager", initInst.Manager().PublicKey.String()),
				zap.String("authority", initInst.Authority().PublicKey.String()),
			)
		}
	}
	return nil
}

type rewardDisbursementsRow struct {
	signature           string
	instructionIndex    int
	amount              uint64
	slot                uint64
	userBank            string
	challengeId         string
	specifier           string
	recipientEthAddress string
}

func insertRewardDisbursement(ctx context.Context, db database.DBTX, row rewardDisbursementsRow) error {
	sql := `
		INSERT INTO sol_reward_disbursements
			(signature, instruction_index, amount, slot, user_bank, challenge_id, specifier, recipient_eth_address)
		VALUES
			(@signature, @instructionIndex, @amount, @slot, @userBank, @challengeId, @specifier, @recipientEthAddress)
		ON CONFLICT DO NOTHING
	;`
	_, err := db.Exec(ctx, sql, pgx.NamedArgs{
		"signature":           row.signature,
		"instructionIndex":    row.instructionIndex,
		"amount":              row.amount,
		"slot":                row.slot,
		"userBank":            row.userBank,
		"challengeId":         row.challengeId,
		"specifier":           row.specifier,
		"recipientEthAddress": row.recipientEthAddress,
	})
	return err
}

type rewardInitRow struct {
	signature          string
	instructionIndex   int
	slot               uint64
	minVotes           uint8
	rewardManagerState string
	tokenSource        string
	mint               string
	manager            string
	authority          string
}

func insertRewardInit(ctx context.Context, db database.DBTX, row rewardInitRow) error {
	sql := `
		INSERT INTO sol_reward_manager_inits
			(signature, instruction_index, slot, min_votes, reward_manager_state, token_source, mint, manager, authority)
		VALUES
			(@signature, @instructionIndex, @slot, @minVotes, @rewardManagerState, @tokenSource, @mint, @manager, @authority)
		ON CONFLICT DO NOTHING
	;`
	_, err := db.Exec(ctx, sql, pgx.NamedArgs{
		"signature":          row.signature,
		"instructionIndex":   row.instructionIndex,
		"slot":               row.slot,
		"minVotes":           row.minVotes,
		"rewardManagerState": row.rewardManagerState,
		"tokenSource":        row.tokenSource,
		"mint":               row.mint,
		"manager":            row.manager,
		"authority":          row.authority,
	})
	return err
}
