package api

import (
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"
)

type WithdrawalForDownload struct {
	DestinationWallet string    `db:"destination_wallet" json:"destination_wallet" csv:"destination wallet"`
	Date              time.Time `db:"date" json:"date" csv:"date"`
	Amount            float64   `db:"amount" json:"amount" csv:"amount"`
}

func (app *ApiServer) userWithdrawalsForDownload(c *fiber.Ctx) ([]WithdrawalForDownload, error) {
	userId := app.getUserId(c)

	// Sourced from the same sol_* tables that v_token_transactions_history uses,
	// but joined directly to sol_claimable_account_transfers so we can surface
	// the destination wallet (cat.to_account) — the view's tx_metadata is NULL
	// for transfers, so it can't be used. Until the Go indexer adds a
	// sol_withdrawals table this also includes any USDC send between
	// user_banks (rare in practice).
	sql := `
		SELECT
			cat.to_account AS destination_wallet,
			bc.block_timestamp AS date,
			ABS(bc.change)::numeric / 1000000 AS amount
		FROM users
		JOIN sol_claimable_accounts sca
			ON sca.ethereum_address = users.wallet
		   AND sca.mint = @usdc_mint
		JOIN sol_token_account_balance_changes bc
			ON bc.account = sca.account
		   AND bc.mint = sca.mint
		JOIN sol_claimable_account_transfers cat
			ON cat.signature = bc.signature
		   AND cat.from_account = bc.account
		WHERE users.user_id = @userId
		  AND users.is_current = TRUE
		  AND bc.change < 0
		ORDER BY bc.block_timestamp DESC;`

	rows, err := app.pool.Query(c.Context(), sql, pgx.NamedArgs{
		"userId":    userId,
		"usdc_mint": usdcMint,
	})
	if err != nil {
		return nil, err
	}

	withdrawals, err := pgx.CollectRows(rows, pgx.RowToStructByName[WithdrawalForDownload])
	if err != nil {
		return nil, err
	}

	return withdrawals, nil
}

func (app *ApiServer) v1UsersWithdrawalsDownloadJson(c *fiber.Ctx) error {
	withdrawals, err := app.userWithdrawalsForDownload(c)
	if err != nil {
		return err
	}

	return c.JSON(fiber.Map{
		"data": withdrawals,
	})
}

func (app *ApiServer) v1UsersWithdrawalsDownloadCsv(c *fiber.Ctx) error {
	withdrawals, err := app.userWithdrawalsForDownload(c)
	if err != nil {
		return err
	}

	c.Set("Content-Type", "text/csv")
	c.Set("Content-Disposition", "attachment; filename=\"withdrawals.csv\"")

	headers := []string{
		"destination wallet",
		"date",
		"amount",
	}

	csvContent, err := WriteCSVFromStructs(withdrawals, headers)
	if err != nil {
		return err
	}

	return c.SendString(csvContent)
}
