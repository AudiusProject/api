package indexer

import (
	"context"
	"time"

	"bridgerton.audius.co/database"
	"github.com/jackc/pgx/v5"
)

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
	Owner            string
	Mint             string
	PreTokenBalance  uint64
	PostTokenBalance uint64
	Change           int64
}

type balanceChangeRow struct {
	balanceChange
	account        string
	signature      string
	slot           uint64
	blockTimestamp time.Time
}

func insertBalanceChange(ctx context.Context, db database.DBTX, row balanceChangeRow) error {
	sql := `INSERT INTO sol_token_account_balance_changes (owner, account, mint, change, balance, signature, slot, block_timestamp)
						VALUES (@owner, @account, @mint, @change, @balance, @signature, @slot, @blockTimestamp)
						ON CONFLICT DO NOTHING`
	_, err := db.Exec(ctx, sql, pgx.NamedArgs{
		"account":        row.account,
		"owner":          row.balanceChange.Owner,
		"mint":           row.balanceChange.Mint,
		"change":         row.balanceChange.Change,
		"balance":        row.balanceChange.PostTokenBalance,
		"signature":      row.signature,
		"slot":           row.slot,
		"blockTimestamp": row.blockTimestamp.UTC(),
	})
	return err
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

func insertSlotCheckpoint(ctx context.Context, db database.DBTX, slot uint64) error {
	sql := `
		INSERT INTO sol_slot_checkpoint (slot) 
		VALUES (@slot) 
		ON CONFLICT (id) DO UPDATE SET slot = EXCLUDED.slot
		;`
	_, err := db.Exec(ctx, sql, pgx.NamedArgs{"slot": slot})
	return err
}
