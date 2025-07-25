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

	/*
	 * The bulk of this query is calculating the member changes for the last
	 * 24 hours. It calculates how many balances went from 0 to >0 (new members)
	 * and how many went from >0 to 0 (members lost) for both userbank and associated wallets.
	 * It then combines these counts to get the net member changes.
	 * Finally, it calculates the total number of members and the percentage change
	 * in members over the last 24 hours (net changes / (current members + net changes)).
	 */
	sql := `
		WITH member_changes_userbank AS (
			SELECT
				sol_token_account_balance_changes.mint,
				(
					COUNT(DISTINCT users.user_id) 
						FILTER (WHERE balance > 0 AND change = balance)
				 	- COUNT(DISTINCT users.user_id) 
						FILTER (WHERE balance = 0 AND change < 0)
				) AS net_new_members
			FROM sol_token_account_balance_changes
			JOIN sol_claimable_accounts
				ON sol_claimable_accounts.account = sol_token_account_balance_changes.account
			JOIN users
				ON users.wallet = sol_claimable_accounts.ethereum_address
			WHERE block_timestamp > NOW() - INTERVAL '24 hours'
				AND sol_token_account_balance_changes.mint = @mint
			GROUP BY sol_token_account_balance_changes.mint
		), member_changes_associated_wallets AS (
			SELECT
				sol_token_account_balance_changes.mint,
				(
					COUNT(DISTINCT associated_wallets.user_id) 
						FILTER (WHERE balance > 0 AND change = balance)
				 	- COUNT(DISTINCT associated_wallets.user_id) 
						FILTER (WHERE balance = 0 AND change < 0)
				) AS net_new_members
			FROM sol_token_account_balance_changes
			JOIN associated_wallets
				ON associated_wallets.wallet = sol_token_account_balance_changes.owner
				AND associated_wallets.chain = 'sol'
			WHERE block_timestamp > NOW() - INTERVAL '24 hours'
				AND sol_token_account_balance_changes.mint = @mint
			GROUP BY sol_token_account_balance_changes.mint	
		), net_member_changes AS (
			SELECT
				member_changes.mint,
				COALESCE(SUM(member_changes.net_new_members), 0) AS change
			FROM (
				SELECT * FROM member_changes_userbank
				UNION ALL SELECT * FROM member_changes_associated_wallets
			) member_changes
			GROUP BY member_changes.mint
		), member_counts AS (
			SELECT
				member_balances.mint,
				COUNT(DISTINCT member_balances.user_id) AS members
			FROM (
				SELECT sol_token_account_balances.mint,
					COALESCE(associated_wallets.user_id, users.user_id) AS user_id
				FROM sol_token_account_balances
				LEFT JOIN associated_wallets 
					ON associated_wallets.wallet = sol_token_account_balances.owner
				LEFT JOIN sol_claimable_accounts 
					ON sol_claimable_accounts.account = sol_token_account_balances.account
				LEFT JOIN users 
					ON users.wallet = sol_claimable_accounts.ethereum_address
				WHERE sol_token_account_balances.balance > 0
					AND (associated_wallets.wallet IS NOT NULL 
							OR sol_claimable_accounts.account IS NOT NULL)
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
			COALESCE(
				(net_member_changes.change * 100.0) / 
				NULLIF(
					COALESCE(member_counts.members, 0) - 
					COALESCE(net_member_changes.change, 0)
				, 0)
			, 0) AS members_24h_change_percent
		FROM artist_coins
		LEFT JOIN member_counts ON artist_coins.mint = member_counts.mint
		LEFT JOIN net_member_changes ON artist_coins.mint = net_member_changes.mint
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
