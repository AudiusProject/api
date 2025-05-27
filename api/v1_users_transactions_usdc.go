package api

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"bridgerton.audius.co/api/dbv1"
)

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
	Limit                     int32    `query:"limit"`
	Offset                    int32    `query:"offset"`
	SortMethod                string   `query:"sort_method"`
	SortDirection             string   `query:"sort_direction"`
	IncludeSystemTransactions bool     `query:"include_system_transactions"`
	TransactionMethod         string   `query:"method"`
}

var validTransactionTypes = []string{
	"purchase_content",
	"transfer",
	"prepare_withdrawal",
	"recover_withdrawal",
	"withdrawal",
	"purchase_stripe",
}

var validTransactionMethods = []string{
	"send",
	"receive",
}

func (app *ApiServer) v1UsersTransactionsUsdc(c *fiber.Ctx) error {
	queryParams := GetUsdcTransactionsParams{}
	if err := c.QueryParser(&queryParams); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "Invalid query parameters")
	}

	// Default values
	if queryParams.Limit == 0 {
		queryParams.Limit = 100
	}

	if queryParams.SortMethod == "" {
		queryParams.SortMethod = "date"
	}

	if queryParams.SortDirection == "" {
		queryParams.SortDirection = "desc"
	}

	if queryParams.SortMethod != "date" && queryParams.SortMethod != "transaction_type" {
		return fiber.NewError(fiber.StatusBadRequest, "Invalid sort method")
	}
	if queryParams.SortDirection != "asc" && queryParams.SortDirection != "desc" {
		return fiber.NewError(fiber.StatusBadRequest, "Invalid sort direction")
	}

	filters := []string{"users.is_current = TRUE"}

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
		if !slices.Contains(validTransactionMethods, queryParams.TransactionMethod) {
			return fiber.NewError(fiber.StatusBadRequest, "Invalid transaction method")
		}
		filters = append(filters, `method = @transaction_method`)
	}

	var orderBy string
	var sortDirection string
	switch queryParams.SortDirection {
	case "asc":
		sortDirection = "asc"
	case "desc":
		sortDirection = "desc"
	}

	switch queryParams.SortMethod {
	case "date":
		orderBy = fmt.Sprintf("uth.created_at %s", sortDirection)
	case "transaction_type":
		orderBy = fmt.Sprintf("transaction_type %s, uth.created_at desc", sortDirection)
	}

	sql := `
	SELECT uth.created_at as transaction_date, transaction_type, uth.signature, method, uth.user_bank, tx_metadata as metadata, change::text, balance::text
	FROM users
	JOIN usdc_user_bank_accounts uba ON uba.ethereum_address = users.wallet
	JOIN usdc_transactions_history uth ON uth.user_bank = uba.bank_account
	WHERE users.user_id = @user_id::int
	AND ` + strings.Join(filters, " AND ") + `
	ORDER BY ` + orderBy + `
	LIMIT @limit_val
	OFFSET @offset_val;
	`

	params := pgx.NamedArgs{
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
	// TODO: Migrate from sqlc, add method, type, include_system_transactions filtering
	count, err := app.queries.GetUserUsdcTransactionsCount(c.Context(), dbv1.GetUserUsdcTransactionsCountParams{
		UserID: app.getUserId(c),
	})
	if err != nil {
		return err
	}

	return c.JSON(fiber.Map{
		"data": count,
	})
}
