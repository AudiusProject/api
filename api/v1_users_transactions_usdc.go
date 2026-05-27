package api

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// USDC mint on mainnet — used to filter v_token_transactions_history.
const usdcMint = "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v"

type UsdcTransaction struct {
	TransactionDate time.Time   `json:"transaction_date"`
	TransactionType string      `json:"transaction_type"`
	Signature       string      `json:"signature"`
	Method          string      `json:"method"`
	UserBank        string      `json:"user_bank"`
	Metadata        pgtype.Text `json:"metadata"`
	Change          string      `json:"change"`
	Balance         string      `json:"balance"`
}

type GetUsdcTransactionsParams struct {
	TransactionTypes          []string `query:"type"`
	Limit                     int      `query:"limit" default:"100" validate:"min=1,max=100"`
	Offset                    int      `query:"offset" default:"0" validate:"min=0"`
	SortMethod                string   `query:"sort_method" default:"date" validate:"oneof=date transaction_type"`
	SortDirection             string   `query:"sort_direction" default:"desc" validate:"oneof=asc desc"`
	IncludeSystemTransactions bool     `query:"include_system_transactions" default:""`
	TransactionMethod         string   `query:"method" default:"" validate:"omitempty,oneof=send receive"`
}

// Legacy `usdc_transactions_history.transaction_type` values still accepted on
// the query string. Values not derivable from v_token_transactions_history
// (purchase_stripe, internal_transfer, prepare_withdrawal, recover_withdrawal,
// withdrawal) will simply match zero rows until the Go indexer grows vendor
// memo capture + a sol_withdrawals table.
var validTransactionTypes = []string{
	"purchase_content",
	"transfer",
	"internal_transfer",
	"prepare_withdrawal",
	"recover_withdrawal",
	"withdrawal",
	"purchase_stripe",
}

func (app *ApiServer) v1UsersTransactionsUsdc(c *fiber.Ctx) error {
	queryParams := GetUsdcTransactionsParams{}
	if err := app.ParseAndValidateQueryParams(c, &queryParams); err != nil {
		return err
	}

	filters := []string{"mint = @mint", "user_id = @user_id::int"}

	transactionTypes := queryParams.TransactionTypes
	if len(transactionTypes) > 0 {
		for _, transactionType := range transactionTypes {
			if !slices.Contains(validTransactionTypes, transactionType) {
				return fiber.NewError(fiber.StatusBadRequest, "Invalid transaction type")
			}
		}
		filters = append(filters, `transaction_type = ANY(@transaction_types::text[])`)
	}

	if !queryParams.IncludeSystemTransactions {
		filters = append(filters, `transaction_type NOT IN ('prepare_withdrawal', 'recover_withdrawal')`)
	}

	if queryParams.TransactionMethod != "" {
		filters = append(filters, `method = @transaction_method`)
	}

	var sortDirection = "desc"
	if queryParams.SortDirection == "asc" {
		sortDirection = "asc"
	}

	var orderBy string
	switch queryParams.SortMethod {
	case "date":
		orderBy = fmt.Sprintf("transaction_date %s", sortDirection)
	case "transaction_type":
		orderBy = fmt.Sprintf("transaction_type %s, transaction_date desc", sortDirection)
	}

	sql := `
	SELECT transaction_date, transaction_type, signature, method, user_bank, tx_metadata AS metadata, change, balance
	FROM v_token_transactions_history
	WHERE ` + strings.Join(filters, " AND ") + `
	ORDER BY ` + orderBy + `
	LIMIT @limit_val
	OFFSET @offset_val;
	`

	params := pgx.NamedArgs{
		"mint":               usdcMint,
		"user_id":            app.getUserId(c),
		"transaction_types":  transactionTypes,
		"limit_val":          queryParams.Limit,
		"offset_val":         queryParams.Offset,
		"transaction_method": queryParams.TransactionMethod,
	}

	rows, err := app.pool.Query(c.Context(), sql, params)
	if err != nil {
		return err
	}

	transactions, err := pgx.CollectRows(rows, pgx.RowToStructByName[UsdcTransaction])
	if err != nil {
		return err
	}

	return c.JSON(fiber.Map{
		"data": transactions,
	})
}

func (app *ApiServer) v1UsersTransactionsUsdcCount(c *fiber.Ctx) error {
	queryParams := GetUsdcTransactionsParams{}
	if err := app.ParseAndValidateQueryParams(c, &queryParams); err != nil {
		return err
	}
	filters := []string{"mint = @mint", "user_id = @user_id::int"}

	transactionTypes := queryParams.TransactionTypes
	if len(transactionTypes) > 0 {
		for _, transactionType := range transactionTypes {
			if !slices.Contains(validTransactionTypes, transactionType) {
				return fiber.NewError(fiber.StatusBadRequest, "Invalid transaction type")
			}
		}
		filters = append(filters, `transaction_type = ANY(@transaction_types::text[])`)
	}

	if !queryParams.IncludeSystemTransactions {
		filters = append(filters, `transaction_type NOT IN ('prepare_withdrawal', 'recover_withdrawal')`)
	}

	if queryParams.TransactionMethod != "" {
		filters = append(filters, `method = @transaction_method`)
	}

	sql := `
		SELECT count(*)
		FROM v_token_transactions_history
		WHERE ` + strings.Join(filters, " AND ") + `;`

	params := pgx.NamedArgs{
		"mint":               usdcMint,
		"user_id":            app.getUserId(c),
		"transaction_types":  transactionTypes,
		"transaction_method": queryParams.TransactionMethod,
	}

	row := app.pool.QueryRow(c.Context(), sql, params)

	var count int64
	err := row.Scan(&count)
	if err != nil {
		return err
	}

	return c.JSON(fiber.Map{
		"data": count,
	})
}
