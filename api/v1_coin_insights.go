package api

import (
	"bridgerton.audius.co/api/birdeye"
	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"
)

type ArtistCoinInsights struct {
	birdeye.TokenOverview
	MembersStatsRow
}

type MembersStatsRow struct {
	Members                 int     `json:"members"`
	Members24hChangePercent float64 `json:"membersChange24hPercent"`
}

func (app *ApiServer) v1CoinInsights(c *fiber.Ctx) error {
	mint := c.Params("mint")
	if mint == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "mint parameter is required",
		})
	}

	/*
	 * The bulk of this query is calculating the member changes for the last 24h.
	 *
	 * t_balance_changes
	 * 		collects all balance changes for the last 24h for the specified mint.
	 * t_user_balance_changes
	 * 		connects t_balance_changes to users via associated_wallets and
	 * 		sol_claimable_accounts, summing the changes for each user.
	 * t_user_balances
	 * 		collects the current balances by user for the mint.
	 * member_changes
	 * 		calculates the net member changes by counting how many user balances
	 * 		went from 0 to >0 (new members) and how many went from >0 to 0
	 * 		(members lost) for each mint.
	 * members
	 * 		calculates the total number of members for each mint by counting distinct
	 * 		user_ids with a balance > 0
	 * Finally, the main query selects the artist coins and joins the member counts
	 * and member changes, calculating the percentage change in members over the last 24h.
	 */
	sql := `
		WITH 
		t_balance_changes AS (
			SELECT
				account,
				owner,
				change,
				balance
			FROM sol_token_account_balance_changes
			WHERE block_timestamp > NOW() - INTERVAL '24 hours'
				AND sol_token_account_balance_changes.mint = @mint
		),
		t_user_balance_changes AS (
			SELECT 
				COALESCE(users.user_id, associated_wallets.user_id) AS user_id,
				SUM(change) AS change
			FROM t_balance_changes	
			LEFT JOIN sol_claimable_accounts
				ON sol_claimable_accounts.account = t_balance_changes.account
			LEFT JOIN users
				ON users.wallet = sol_claimable_accounts.ethereum_address
			LEFT JOIN associated_wallets
				ON associated_wallets.wallet = t_balance_changes.owner
				AND associated_wallets.chain = 'sol'
			WHERE users.user_id IS NOT NULL OR associated_wallets.user_id IS NOT NULL
			GROUP BY COALESCE(users.user_id, associated_wallets.user_id)
		),
		t_user_balances AS (
			SELECT 
				user_id,
				balance
			FROM sol_user_balances
			WHERE mint = @mint			
		),
		member_changes AS (
			SELECT
				(
					COUNT(DISTINCT t_user_balance_changes.user_id) 
						FILTER (WHERE change = balance AND balance > 0) -
					COUNT(DISTINCT t_user_balance_changes.user_id) 
						FILTER (WHERE change < 0 AND balance = 0)
				) AS net
			FROM t_user_balance_changes
			JOIN sol_user_balances
				ON t_user_balance_changes.user_id = sol_user_balances.user_id
		),
		members AS (
			SELECT
				COUNT(DISTINCT user_id) AS count
			FROM t_user_balances
			WHERE balance > 0
		)
		SELECT 
			COALESCE(members.count, 0) AS members,
			COALESCE(
				(member_changes.net * 100.0) / 
				NULLIF(
					COALESCE(members.count, 0) - 
					COALESCE(member_changes.net, 0)
				, 0)
			, 0) AS members_24h_change_percent
		FROM members, member_changes
	`

	rows, err := app.pool.Query(c.Context(), sql, pgx.NamedArgs{
		"mint": mint,
	})
	if err != nil {
		return err
	}

	membersStatsRows, err := pgx.CollectExactlyOneRow(rows, pgx.RowToStructByName[MembersStatsRow])
	if err != nil {
		return err
	}

	overview, err := app.birdeyeClient.GetTokenOverview(c.Context(), mint, "24h")
	if err != nil {
		return err
	}
	if overview == nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Token overview not found",
		})
	}

	insights := ArtistCoinInsights{
		MembersStatsRow: membersStatsRows,
		TokenOverview:   *overview,
	}

	return c.JSON(fiber.Map{
		"data": insights,
	})
}
