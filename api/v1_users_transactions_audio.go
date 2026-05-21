package api

import (
	"fmt"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// wAUDIO mint on mainnet — used to filter v_token_transactions_history.
const wAudioMint = "9LzCMqDgTKYz9Drzqnpgee3SGa89up3a247ypMj2xrqM"

type GetUsersTransactionsAudioParams struct {
	Limit         int    `query:"limit" default:"100" validate:"min=1,max=100"`
	Offset        int    `query:"offset" default:"0" validate:"min=0"`
	Sort          string `query:"sort" default:"date" validate:"oneof=date transaction_type"`
	SortDirection string `query:"sort_direction" default:"desc" validate:"oneof=asc desc"`
}

type AudioTransaction struct {
	TransactionDate time.Time   `json:"transaction_date"`
	TransactionType string      `json:"transaction_type"`
	Signature       string      `json:"signature"`
	Method          string      `json:"method"`
	UserBank        string      `json:"user_bank"`
	Metadata        pgtype.Text `json:"metadata"`
	Change          string      `json:"change"`
	Balance         string      `json:"balance"`
}

func (app *ApiServer) v1UsersTransactionsAudio(c *fiber.Ctx) error {
	params := GetUsersTransactionsAudioParams{}
	if err := app.ParseAndValidateQueryParams(c, &params); err != nil {
		return err
	}

	var sortDirection = "desc"
	if params.SortDirection == "asc" {
		sortDirection = "asc"
	}

	var orderBy = fmt.Sprintf("transaction_date %s", sortDirection)
	if params.Sort == "transaction_type" {
		orderBy = fmt.Sprintf("transaction_type %s, transaction_date desc", sortDirection)
	}

	sql := `
		SELECT transaction_date, transaction_type, signature, method, user_bank, tx_metadata AS metadata, change, balance
		FROM v_token_transactions_history
		WHERE mint = @mint AND user_id = @user_id::int
		ORDER BY ` + orderBy + `
		LIMIT @limit_val
		OFFSET @offset_val;
	`

	args := pgx.NamedArgs{
		"mint":       wAudioMint,
		"user_id":    app.getUserId(c),
		"limit_val":  params.Limit,
		"offset_val": params.Offset,
	}

	rows, err := app.pool.Query(c.Context(), sql, args)
	if err != nil {
		return err
	}

	transactions, err := pgx.CollectRows(rows, pgx.RowToStructByName[AudioTransaction])
	if err != nil {
		return err
	}

	return c.JSON(fiber.Map{
		"data": transactions,
	})
}

func (app *ApiServer) v1UsersTransactionsAudioCount(c *fiber.Ctx) error {
	sql := `
		SELECT count(*)
		FROM v_token_transactions_history
		WHERE mint = @mint AND user_id = @user_id::int;
	`

	row := app.pool.QueryRow(c.Context(), sql, pgx.NamedArgs{
		"mint":    wAudioMint,
		"user_id": app.getUserId(c),
	})

	var count int64
	err := row.Scan(&count)
	if err != nil {
		return err
	}

	return c.JSON(fiber.Map{
		"data": count,
	})
}
