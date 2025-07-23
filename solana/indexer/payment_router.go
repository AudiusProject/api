package indexer

import (
	"context"
	"fmt"
	"time"

	"bridgerton.audius.co/config"
	"bridgerton.audius.co/database"
	"bridgerton.audius.co/solana/spl/programs/payment_router"
	"github.com/gagliardetto/solana-go"
	"github.com/jackc/pgx/v5"
	"go.uber.org/zap"
)

func processPaymentRouterInstruction(
	ctx context.Context,
	db database.DBTX,
	slot uint64,
	tx *solana.Transaction,
	instructionIndex int,
	instruction solana.CompiledInstruction,
	signature string,
	blockTime time.Time,
	config config.Config,
	instLogger *zap.Logger,
) error {

	accounts, err := instruction.ResolveInstructionAccounts(&tx.Message)
	if err != nil {
		return fmt.Errorf("error resolving instruction accounts: %w", err)
	}
	inst, err := payment_router.DecodeInstruction(accounts, []byte(instruction.Data))
	if err != nil {
		return fmt.Errorf("error decoding payment_router instruction: %w", err)
	}
	switch inst.TypeID {
	case payment_router.InstructionImplDef.TypeID(payment_router.Instruction_Route):
		if routeInst, ok := inst.Impl.(*payment_router.Route); ok {
			for i, account := range routeInst.GetDestinations() {
				err = insertPayment(ctx, db, paymentRow{
					signature:        signature,
					instructionIndex: instructionIndex,
					amount:           routeInst.Amounts[i],
					slot:             slot,
					routeIndex:       i,
					toAccount:        account.PublicKey.String(),
				})
				if err != nil {
					return fmt.Errorf("failed to insert payment at instruction: %w", err)
				}
			}

			parsedPurchaseMemo, ok := findNextPurchaseMemo(tx, instructionIndex, instLogger)
			if ok {
				parsedLocationMemo := findNextLocationMemo(tx, instructionIndex, instLogger)
				isValid, err := validatePurchase(ctx, config, db, routeInst, parsedPurchaseMemo, blockTime)
				if err != nil {
					instLogger.Error("invalid purchase", zap.Error(err))
					// continue - insert the purchase as invalid for record keeping
				}

				err = insertPurchase(ctx, db, purchaseRow{
					signature:          signature,
					instructionIndex:   instructionIndex,
					amount:             routeInst.TotalAmount,
					slot:               slot,
					fromAccount:        routeInst.GetSender().PublicKey.String(),
					parsedPurchaseMemo: parsedPurchaseMemo,
					parsedLocationMemo: parsedLocationMemo,
					isValid:            isValid,
				})
				if err != nil {
					return fmt.Errorf("failed to insert purchase at instruction: %w", err)
				}
				instLogger.Info("payment_router purchase",
					zap.String("contentType", parsedPurchaseMemo.ContentType),
					zap.Int("contentId", parsedPurchaseMemo.ContentId),
					zap.Int("validAfterBlocknumber", parsedPurchaseMemo.ValidAfterBlocknumber),
					zap.Int("buyerUserId", parsedPurchaseMemo.BuyerUserId),
					zap.String("accessType", parsedPurchaseMemo.AccessType),
				)
			}

			instLogger.Info("payment_router route",
				zap.String("sender", routeInst.GetSender().PublicKey.String()),
				zap.Uint64s("amounts", routeInst.Amounts),
				zap.Strings("destinations", routeInst.GetDestinations().GetKeys().ToBase58()),
			)
		}
	}
	return nil
}

type purchaseRow struct {
	signature        string
	instructionIndex int
	amount           uint64
	slot             uint64
	fromAccount      string

	parsedPurchaseMemo
	parsedLocationMemo

	isValid *bool
}

func insertPurchase(ctx context.Context, db database.DBTX, row purchaseRow) error {
	sql := `
	INSERT INTO sol_purchases 
		(signature, instruction_index, amount, slot, from_account, content_type, content_id, buyer_user_id, access_type, valid_after_blocknumber, is_valid, city, region, country)
	VALUES
		(@signature, @instructionIndex, @amount, @slot, @fromAccount, @contentType, @contentId, @buyerUserId, @accessType, @validAfterBlocknumber, @isValid, @city, @region, @country)
	ON CONFLICT DO NOTHING
	;`

	_, err := db.Exec(ctx, sql, pgx.NamedArgs{
		"signature":             row.signature,
		"instructionIndex":      row.instructionIndex,
		"amount":                row.amount,
		"slot":                  row.slot,
		"fromAccount":           row.fromAccount,
		"contentType":           row.ContentType,
		"contentId":             row.ContentId,
		"buyerUserId":           row.BuyerUserId,
		"accessType":            row.AccessType,
		"validAfterBlocknumber": row.ValidAfterBlocknumber,
		"isValid":               row.isValid,
		"city":                  row.City,
		"region":                row.Region,
		"country":               row.Country,
	})
	return err
}

type paymentRow struct {
	signature        string
	instructionIndex int
	amount           uint64
	slot             uint64
	routeIndex       int
	toAccount        string
}

func insertPayment(ctx context.Context, db database.DBTX, row paymentRow) error {
	sql := `
	INSERT INTO sol_payments
		(signature, instruction_index, amount, slot, route_index, to_account)
	VALUES
		(@signature, @instructionIndex, @amount, @slot, @routeIndex, @toAccount)
	ON CONFLICT DO NOTHING
	;`
	_, err := db.Exec(ctx, sql, pgx.NamedArgs{
		"signature":        row.signature,
		"instructionIndex": row.instructionIndex,
		"amount":           row.amount,
		"slot":             row.slot,
		"routeIndex":       row.routeIndex,
		"toAccount":        row.toAccount,
	})
	return err
}
