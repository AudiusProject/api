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

	// See v1_coins for the query explanation.
	sql := `
		WITH t_user_balance_changes AS (
			SELECT
				sol_token_account_balance_changes.mint,
				users.user_id,
				change,
				0 AS balance
			FROM sol_token_account_balance_changes
			JOIN sol_claimable_accounts
				ON sol_claimable_accounts.account = sol_token_account_balance_changes.account
			JOIN users
				ON users.wallet = sol_claimable_accounts.ethereum_address
			WHERE block_timestamp > NOW() - INTERVAL '24 hours'
				AND sol_token_account_balance_changes.mint = @mint
			UNION ALL
			SELECT
				sol_token_account_balance_changes.mint,
				associated_wallets.user_id,
				change,
				0 AS balance
			FROM sol_token_account_balance_changes
			JOIN associated_wallets
				ON associated_wallets.wallet = sol_token_account_balance_changes.owner
				AND associated_wallets.chain = 'sol'	
			WHERE block_timestamp > NOW() - INTERVAL '24 hours'
				AND sol_token_account_balance_changes.mint = @mint
		), t_user_balances AS (
			SELECT
				sol_token_account_balances.mint,
				associated_wallets.user_id,
				0 AS change,
				sol_token_account_balances.balance
			FROM sol_token_account_balances
			JOIN associated_wallets 
				ON associated_wallets.wallet = sol_token_account_balances.owner
				AND associated_wallets.chain = 'sol'
			WHERE sol_token_account_balances.mint = @mint
			UNION ALL
			SELECT
				sol_token_account_balances.mint,
				users.user_id,
				0 AS change,
				sol_token_account_balances.balance
			FROM sol_token_account_balances
			JOIN sol_claimable_accounts
				ON sol_claimable_accounts.account = sol_token_account_balances.account
			JOIN users 
				ON users.wallet = sol_claimable_accounts.ethereum_address
			WHERE sol_token_account_balances.mint = @mint
		), total_user_balance_changes AS (
			SELECT
				t.mint,
				t.user_id,
				SUM(t.change) AS change,
				SUM(t.balance) AS balance
			FROM (
				SELECT * FROM t_user_balance_changes 
				UNION ALL 
				SELECT * FROM t_user_balances
			) t
			GROUP BY
				t.mint,
				t.user_id
		), member_changes AS (
			SELECT
				mint,
				(
					COUNT(DISTINCT user_id) FILTER (WHERE change = balance AND balance > 0) -
					COUNT(DISTINCT user_id) FILTER (WHERE change < 0 AND balance = 0)
				) AS net
			FROM total_user_balance_changes
			GROUP BY 
				mint
		), members AS (
			SELECT
				mint,
				COUNT(DISTINCT user_id) AS count
			FROM t_user_balances
			WHERE balance > 0
			GROUP BY mint
		)
		SELECT 
			artist_coins.ticker,
			artist_coins.mint,
			artist_coins.decimals,
			artist_coins.user_id,
			artist_coins.created_at,
			COALESCE(members.count, 0) AS members,
			COALESCE(
				(member_changes.net * 100.0) / 
				NULLIF(
					COALESCE(members.count, 0) - 
					COALESCE(member_changes.net, 0)
				, 0)
			, 0) AS members_24h_change_percent
		FROM artist_coins
		LEFT JOIN members ON artist_coins.mint = members.mint
		LEFT JOIN member_changes ON artist_coins.mint = member_changes.mint
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
