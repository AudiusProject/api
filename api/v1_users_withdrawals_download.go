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

	sql := `
		SELECT
			uth.tx_metadata AS destination_wallet,
			uth.transaction_created_at AS date,
			ABS(uth.change) / 1000000 AS amount
		FROM users
		JOIN usdc_user_bank_accounts uba ON uba.ethereum_address = users.wallet
		JOIN usdc_transactions_history uth ON uth.user_bank = uba.bank_account
		WHERE users.user_id = @userId
		AND users.is_current = TRUE
		AND uth.transaction_type = 'transfer'
		AND uth.method = 'send'
		ORDER BY uth.transaction_created_at DESC;`

	rows, err := app.pool.Query(c.Context(), sql, pgx.NamedArgs{
		"userId": userId,
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
