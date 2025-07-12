package indexer

import (
	"context"

	"bridgerton.audius.co/database"
	"github.com/jackc/pgx/v5"
	"go.uber.org/zap"
)

type claimableAccountsRow struct {
	signature        string
	instructionIndex int
	slot             uint64
	mint             string
	ethereumAddress  string
	bankAccount      string
}

func insertClaimableAccount(ctx context.Context, db database.DBTX, row claimableAccountsRow) error {
	sql := `
		INSERT INTO sol_claimable_accounts
			(signature, instruction_index, slot, mint, ethereum_address, bank_account)
		VALUES
			(@signature, @instructionIndex, @slot, @mint, @ethereumAddress, @bankAccount)
		ON CONFLICT DO NOTHING
	;`
	_, err := db.Exec(ctx, sql, pgx.NamedArgs{
		"signature":        row.signature,
		"instructionIndex": row.instructionIndex,
		"slot":             row.slot,
		"mint":             row.mint,
		"ethereumAddress":  row.ethereumAddress,
		"bankAccount":      row.bankAccount,
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

type balanceChange struct {
	Mint             string
	PreTokenBalance  uint64
	PostTokenBalance uint64
	Change           int64
}

type balanceChangeRow struct {
	balanceChange
	account   string
	signature string
	slot      uint64
}

func insertBalanceChange(ctx context.Context, db database.DBTX, row balanceChangeRow, logger *zap.Logger) error {
	sql := `INSERT INTO solana_token_txs (account_address, mint, change, balance, signature, slot)
						VALUES (@account_address, @mint, @change, @balance, @signature, @slot)
						ON CONFLICT DO NOTHING`
	_, err := db.Exec(ctx, sql, pgx.NamedArgs{
		"account_address": row.account,
		"mint":            row.balanceChange.Mint,
		"change":          row.balanceChange.Change,
		"balance":         row.balanceChange.PostTokenBalance,
		"signature":       row.signature,
		"slot":            row.slot,
	})
	if logger != nil {
		logger.Debug("inserting balance change...",
			zap.String("account", row.account),
			zap.String("mint", row.balanceChange.Mint),
			zap.Uint64("balance", row.balanceChange.PostTokenBalance),
			zap.Int64("change", row.balanceChange.Change),
			zap.Uint64("slot", row.slot),
		)
	}
	return err
}

type parsedPurchaseMemo struct {
	contentType           string
	contentId             int
	validAfterBlocknumber int
	buyerUserId           int
	accessType            string
}

type parsedLocationMemo struct {
	City    string `json:"city"`
	Region  string `json:"region"`
	Country string `json:"country"`
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
	ON CONFLICT DO NOTHIN
	;`

	_, err := db.Exec(ctx, sql, pgx.NamedArgs{
		"signature":             row.signature,
		"instructionIndex":      row.instructionIndex,
		"amount":                row.amount,
		"slot":                  row.slot,
		"fromAccount":           row.fromAccount,
		"contentType":           row.contentType,
		"contentId":             row.contentId,
		"buyerUserId":           row.buyerUserId,
		"accessType":            row.accessType,
		"validAfterBlocknumber": row.validAfterBlocknumber,
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
	`
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
