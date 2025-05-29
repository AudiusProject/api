package api

import (
	"fmt"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

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
	sortMethodQuery := c.Query("sort_method", "date")
	if sortMethodQuery != "date" && sortMethodQuery != "transaction_type" {
		return fiber.NewError(fiber.StatusBadRequest, "Invalid sort method")
	}
	sortDirectionQuery := c.Query("sort_direction", "desc")
	if sortDirectionQuery != "asc" && sortDirectionQuery != "desc" {
		return fiber.NewError(fiber.StatusBadRequest, "Invalid sort direction")
	}

	var orderBy string
	var sortDirection string
	switch sortDirectionQuery {
	case "asc":
		sortDirection = "asc"
	case "desc":
		sortDirection = "desc"
	}

	switch sortMethodQuery {
	case "date":
		orderBy = fmt.Sprintf("ath.created_at %s", sortDirection)
	case "transaction_type":
		orderBy = fmt.Sprintf("transaction_type %s, ath.created_at desc", sortDirection)
	}

	sql := `
		SELECT ath.created_at as transaction_date, transaction_type, ath.signature, method, ath.user_bank, tx_metadata as metadata, change::text, balance::text
		FROM users
		JOIN user_bank_accounts uba ON uba.ethereum_address = users.wallet
		JOIN audio_transactions_history ath ON ath.user_bank = uba.bank_account
		WHERE users.user_id = @user_id::int AND users.is_current = TRUE
		ORDER BY ` + orderBy + `
		LIMIT @limit_val
		OFFSET @offset_val;
	`

	params := pgx.NamedArgs{
		"user_id":    app.getUserId(c),
		"limit_val":  c.QueryInt("limit", 100),
		"offset_val": c.QueryInt("offset", 0),
	}

	rows, err := app.pool.Query(c.Context(), sql, params)
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
		FROM users
		JOIN user_bank_accounts uba ON uba.ethereum_address = users.wallet
		JOIN audio_transactions_history ath ON ath.user_bank = uba.bank_account
		WHERE users.user_id = @user_id::int AND users.is_current = TRUE;
	`

	row := app.pool.QueryRow(c.Context(), sql, pgx.NamedArgs{
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
