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
				COUNT(DISTINCT balances_24h_ago.user_id) AS members,
				balances_24h_ago.mint
			FROM (
				SELECT DISTINCT ON (sol_token_account_balance_changes.mint, sol_token_account_balance_changes.account, users.user_id, associated_wallets.user_id)
					sol_token_account_balance_changes.mint,
					sol_token_account_balance_changes.account,
					sol_token_account_balance_changes.balance,
					COALESCE(associated_wallets.user_id, users.user_id) AS user_id
				FROM sol_token_account_balance_changes
				LEFT JOIN associated_wallets ON associated_wallets.wallet = sol_token_account_balance_changes.owner
				LEFT JOIN sol_claimable_accounts ON sol_claimable_accounts.account = sol_token_account_balance_changes.account
				LEFT JOIN users ON users.wallet = sol_claimable_accounts.ethereum_address
				WHERE block_timestamp < NOW() - INTERVAL '24 hours'
					AND (associated_wallets.user_id IS NOT NULL OR users.user_id IS NOT NULL)
					AND sol_token_account_balance_changes.mint = @mint
				ORDER BY sol_token_account_balance_changes.mint, sol_token_account_balance_changes.account, users.user_id, associated_wallets.user_id, block_timestamp DESC
			) AS balances_24h_ago
			WHERE balance > 0
			GROUP BY balances_24h_ago.mint
		), member_counts AS (
			SELECT
				member_balances.mint,
				COUNT(DISTINCT member_balances.user_id) AS members
			FROM (
				SELECT sol_token_account_balances.mint,
					COALESCE(associated_wallets.user_id, users.user_id) AS user_id
				FROM sol_token_account_balances
				LEFT JOIN associated_wallets ON associated_wallets.wallet = sol_token_account_balances.owner
				LEFT JOIN sol_claimable_accounts ON sol_claimable_accounts.account = sol_token_account_balances.account
				LEFT JOIN users ON users.wallet = sol_claimable_accounts.ethereum_address
				WHERE sol_token_account_balances.balance > 0
					AND (associated_wallets.wallet IS NOT NULL OR sol_claimable_accounts.account IS NOT NULL)
					AND sol_token_account_balances.mint = @mint
			) AS member_balances
			GROUP BY member_balances.mint
		)
		SELECT 
			artist_coins.ticker,
			artist_coins.mint,
			artist_coins.user_id,
			artist_coins.created_at,
			COALESCE(member_counts.members, 0) AS members,
			((COALESCE(member_counts.members, 0) - member_counts_24h_ago.members) * 100.0 / NULLIF(member_counts_24h_ago.members, 0)) AS members_24h_change_percent
		FROM artist_coins
		LEFT JOIN member_counts ON artist_coins.mint = member_counts.mint
		LEFT JOIN member_counts_24h_ago ON artist_coins.mint = member_counts_24h_ago.mint
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
