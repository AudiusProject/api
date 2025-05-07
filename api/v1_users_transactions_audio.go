package api

import (
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgtype"
	"github.com/jackc/pgx/v5"
)

func (app *ApiServer) v1UsersTransactionsAudio(c *fiber.Ctx) error {
	sortMethodQuery := c.Query("sort_method", "date")
	if sortMethodQuery != "date" && sortMethodQuery != "type" {
		return fiber.NewError(fiber.StatusBadRequest, "Invalid sort method")
	}
	sortDirectionQuery := c.Query("sort_direction", "desc")
	if sortDirectionQuery != "asc" && sortDirectionQuery != "desc" {
		return fiber.NewError(fiber.StatusBadRequest, "Invalid sort direction")
	}

	sortMethod := "ath.created_at"
	if sortMethodQuery == "type" {
		sortMethod = "transaction_type"
	}
	sortDirection := "DESC"
	if sortDirectionQuery == "asc" {
		sortDirection = "ASC"
	}

	sql := `
	SELECT ath.created_at, transaction_type, ath.signature, method, ath.user_bank, tx_metadata, change::text, balance::text
	FROM users
	JOIN user_bank_accounts uba ON uba.ethereum_address = users.wallet
	JOIN audio_transactions_history ath ON ath.user_bank = uba.bank_account
	WHERE users.user_id = @user_id::int AND users.is_current = TRUE
	ORDER BY ` + sortMethod + ` ` + sortDirection + `
	LIMIT @limit
	OFFSET @offset`

	args := pgx.NamedArgs{
		"user_id": app.getUserId(c),
	}
	args["limit"] = c.QueryInt("limit", 100)
	args["offset"] = c.QueryInt("offset", 0)

	rows, err := app.pool.Query(c.Context(), sql, args)
	if err != nil {
		return err
	}

	type UserAudioTransaction struct {
		CreatedAt       time.Time   `json:"transaction_date"`
		TransactionType string      `json:"transaction_type"`
		Method          string      `json:"method"`
		UserBank        string      `json:"user_bank"`
		TxMetadata      pgtype.Text `json:"metadata"`
		Signature       string      `json:"signature"`
		Change          string      `json:"change"`
		Balance         string      `json:"balance"`
	}

	transactions, err := pgx.CollectRows(rows, pgx.RowToStructByName[UserAudioTransaction])
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
	JOIN user_bank_accounts ON user_bank_accounts.ethereum_address = users.wallet
	JOIN audio_transactions_history ON audio_transactions_history.user_bank = user_bank_accounts.bank_account
	WHERE users.user_id = @user_id::int AND users.is_current = TRUE`

	rows, err := app.pool.Query(c.Context(), sql, pgx.NamedArgs{
		"user_id": app.getUserId(c),
	})

	count, err := pgx.CollectOneRow(rows, pgx.RowTo[int32])
	if err != nil {
		return err
	}

	return c.JSON(fiber.Map{
		"data": count,
	})
}
