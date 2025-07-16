package api

import (
	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"
)

func (app *ApiServer) v1Coin(c *fiber.Ctx) error {
	mint := c.Params("mint")
	if mint == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "mint parameter is required",
		})
	}

	sql := `
		WITH member_counts_24h_ago AS (
			SELECT 
				COUNT(*) AS members,
				balances_24h_ago.mint
			FROM (
				SELECT DISTINCT ON (sol_token_account_balance_changes.mint, sol_token_account_balance_changes.account)
					sol_token_account_balance_changes.mint,
					sol_token_account_balance_changes.account,
					sol_token_account_balance_changes.balance
				FROM sol_token_account_balance_changes
				WHERE block_timestamp < NOW() - INTERVAL '24 hours'
				ORDER BY sol_token_account_balance_changes.mint, sol_token_account_balance_changes.account, block_timestamp DESC	
			) AS balances_24h_ago
			WHERE balance > 0
			GROUP BY balances_24h_ago.mint
		), member_counts AS (
			SELECT
				sol_token_account_balances.mint,
				COUNT(*) AS members
			FROM sol_token_account_balances
			WHERE sol_token_account_balances.balance > 0
			GROUP BY sol_token_account_balances.mint
		)
		SELECT 
			artist_coins.ticker,
			artist_coins.mint,
			artist_coins.user_id,
			artist_coins.created_at,
			member_counts.members,
			((member_counts.members - member_counts_24h_ago.members) * 100.0 / NULLIF(member_counts_24h_ago.members, 0)) AS members_24h_change_percent
		FROM artist_coins
		JOIN member_counts ON artist_coins.mint = member_counts.mint
		JOIN member_counts_24h_ago ON artist_coins.mint = member_counts_24h_ago.mint
		WHERE artist_coins.mint = @mint
		LIMIT 1
	`

	rows, err := app.pool.Query(c.Context(), sql, pgx.NamedArgs{
		"mint": mint,
	})
	if err != nil {
		return err
	}

	coinRows, err := pgx.CollectExactlyOneRow(rows, pgx.RowToStructByName[ArtistCoin])
	if err != nil {
		return err
	}

	overview, err := app.birdeyeClient.GetTokenOverview(c.Context(), mint, "24h")
	if err != nil {
		return err
	}
	coinRows.TokenInfo = overview

	return c.JSON(fiber.Map{
		"data": coinRows,
	})
}
