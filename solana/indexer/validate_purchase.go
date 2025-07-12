package indexer

import (
	"context"
	"fmt"
	"time"

	"bridgerton.audius.co/api/dbv1"
	"bridgerton.audius.co/config"
	"bridgerton.audius.co/database"
	"bridgerton.audius.co/solana/spl/programs/payment_router"
	"github.com/gagliardetto/solana-go"
	"github.com/jackc/pgx/v5"
)

// Gets the price that should be used for validating a purchase memo.
func getRelevantPrice(ctx context.Context, db database.DBTX, memo parsedPurchaseMemo, timestamp time.Time) (dbv1.PurchaseGate, error) {
	table := "track_price_history"
	idColumn := "track_id"

	if memo.contentType == "album" {
		table = "album_price_history"
		idColumn = "album_id"
	}

	sql := `
		SELECT
			total_price_cents AS price,
			splits
		FROM ` + table + `
		WHERE blocknumber >= @blocknumber
			AND ` + idColumn + ` = @id
			AND access = @accessType
			AND block_timestamp <= @timestamp
		ORDER BY block_timestamp DESC
		LIMIT 1
	`

	rows, err := db.Query(ctx, sql, pgx.NamedArgs{
		"blocknumber": memo.validAfterBlocknumber,
		"id":          memo.contentId,
		"accessType":  memo.accessType,
		"timestamp":   timestamp.UTC(),
	})
	if err != nil {
		return dbv1.PurchaseGate{}, err
	}

	priceRow, err := pgx.CollectExactlyOneRow(rows, pgx.RowToStructByName[dbv1.PurchaseGate])

	return priceRow, err
}

type payoutWalletRow struct {
	UserId       int32  `db:"user_id"`
	PayoutWallet string `db:"payout_wallet"`
}

// Gets the payout wallets for a purchase gate.
// Returns a map of FullUser to satisfy the ToFullPurchaseGate call
// that will almost certainly follow this call.
func getPayoutWallets(ctx context.Context, db database.DBTX, price dbv1.PurchaseGate, timestamp time.Time) (map[int32]dbv1.FullUser, error) {
	sql := `
		WITH max_block_timestamps AS (
			SELECT
				user_id,
				MAX(block_timestamp) AS block_timestamp
			FROM user_payout_wallet_history
			WHERE block_timestamp <= @timestamp
				AND user_id = ANY(@userIds)
			GROUP BY user_id
		)
		SELECT 
			users.user_id,
			COALESCE(
					user_payout_wallet_history.spl_usdc_payout_wallet, 
					usdc_user_bank_accounts.bank_account
				) AS payout_wallet
		FROM users
		LEFT JOIN usdc_user_bank_accounts 
			ON usdc_user_bank_accounts.ethereum_address = users.wallet
		LEFT JOIN max_block_timestamps
			ON max_block_timestamps.user_id = users.user_id
		LEFT JOIN user_payout_wallet_history
			ON user_payout_wallet_history.user_id = max_block_timestamps.user_id
				AND user_payout_wallet_history.block_timestamp = max_block_timestamps.block_timestamp
		WHERE users.user_id = ANY(@userIds)
	`

	userIds := make([]int32, len(price.Splits))
	for i, split := range price.Splits {
		userIds[i] = split.UserID
	}

	rows, err := db.Query(ctx, sql, pgx.NamedArgs{
		"timestamp": timestamp.UTC(),
		"userIds":   userIds,
	})
	if err != nil {
		return nil, err
	}

	payoutWalletRows, err := pgx.CollectRows(rows, pgx.RowToStructByName[payoutWalletRow])
	if err != nil {
		return nil, err
	}

	res := make(map[int32]dbv1.FullUser)
	for _, row := range payoutWalletRows {
		user := dbv1.FullUser{
			GetUsersRow: dbv1.GetUsersRow{
				PayoutWallet: row.PayoutWallet,
			},
		}
		res[row.UserId] = user
	}
	return res, nil
}

// Checks that all the splits got paid out appropriately for a given purchase.
func validatePurchase(ctx context.Context, cfg config.Config, db database.DBTX, inst *payment_router.Route, memo parsedPurchaseMemo, timestamp time.Time) (*bool, error) {
	var currentBlockNumber int
	db.QueryRow(ctx, `SELECT MAX(number) FROM blocks`).Scan(&currentBlockNumber)
	if memo.validAfterBlocknumber > currentBlockNumber {
		// Might not be valid _yet_, needs to wait until Core catches up
		return nil, nil
	}

	ret := false

	relevantPrice, err := getRelevantPrice(ctx, db, memo, timestamp)
	if err != nil {
		return &ret, fmt.Errorf("failed to get relevant price: %w", err)
	}

	payoutWalletMap, err := getPayoutWallets(ctx, db, relevantPrice, timestamp)
	if err != nil {
		return &ret, fmt.Errorf("failed to get payout wallets: %w", err)
	}

	gate := relevantPrice.ToFullPurchaseGate(cfg, payoutWalletMap)

	payments := inst.GetRouteMap()
	for acc, expectedAmt := range gate.Splits {
		key, err := solana.PublicKeyFromBase58(acc)
		if err != nil {
			return &ret, fmt.Errorf("invalid splits %s: %w", acc, err)
		}
		payment := payments[key]
		if payment < uint64(expectedAmt) {
			return &ret, fmt.Errorf("payment for account %s not sufficient (expected %d, received %d)", acc, expectedAmt, payment)
		}
	}
	ret = true
	return &ret, nil
}
