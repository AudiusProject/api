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

	// Scoped to claimable transfers explicitly tagged as `withdrawal` in
	// sol_transfer_memo_types (the program indexer writes these markers when
	// it sees the "Withdrawal" memo). Resolves destination_wallet to the
	// Solana account owner via sol_token_account_balances (the legacy
	// equivalent was Python's receiver_account_owner lookup), falling back
	// to cat.to_account when the destination token account isn't tracked.
	sql := `
		SELECT
			COALESCE(stab.owner, cat.to_account) AS destination_wallet,
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
		JOIN sol_transfer_memo_types tmt
			ON tmt.signature = cat.signature
		   AND tmt.instruction_index = cat.instruction_index
		   AND tmt.memo_type = 'withdrawal'
		LEFT JOIN sol_token_account_balances stab
			ON stab.account = cat.to_account
		   AND stab.mint = sca.mint
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
