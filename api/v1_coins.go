package api

import (
	"sync"
	"time"

	"bridgerton.audius.co/api/birdeye"
	"bridgerton.audius.co/trashid"
	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"
	"go.uber.org/zap"
)

type GetArtistCoinsQueryParams struct {
	Tickers  []string         `query:"ticker"`
	Mints    []string         `query:"mint"`
	OwnerIds []trashid.HashId `query:"owner_id"`
	Limit    int              `query:"limit" default:"50" validate:"min=1,max=100"`
	Offset   int              `query:"offset" default:"0" validate:"min=0"`
}

type ArtistCoin struct {
	Ticker                  string                 `json:"ticker"`
	Mint                    string                 `json:"mint"`
	Decimals                int                    `json:"decimals"`
	OwnerId                 trashid.HashId         `db:"user_id" json:"owner_id"`
	Members                 int                    `json:"members"`
	Members24hChangePercent *float64               `json:"members_24h_change_percent"`
	CreatedAt               time.Time              `json:"created_at"`
	TokenInfo               *birdeye.TokenOverview `db:"-" json:"token_info"`
}

func (app *ApiServer) v1Coins(c *fiber.Ctx) error {
	queryParams := GetArtistCoinsQueryParams{}
	if err := app.ParseAndValidateQueryParams(c, &queryParams); err != nil {
		return err
	}

	mintFilter := ""
	if len(queryParams.Mints) > 0 {
		mintFilter = `AND artist_coins.mint = ANY(@mints)`
	}
	ownerIdFilter := ""
	if len(queryParams.OwnerIds) > 0 {
		ownerIdFilter = `AND artist_coins.user_id = ANY(@owner_ids)`
	}
	tickerFilter := ""
	if len(queryParams.Tickers) > 0 {
		tickerFilter = `AND artist_coins.ticker = ANY(@tickers)`
	}

	/*
	 * The bulk of this query is calculating the member changes for the last 24h.
	 *
	 * t_user_balance_changes
	 * 		collects all balance changes for the last 24h for both userbank and
	 * 		associated wallets and joins to get user_ids.
	 * t_user_balances
	 * 		collects the current balances for both userbank and associated wallets
	 * 		and joins to get user_ids.
	 * total_user_balance_changes
	 * 		sums the 24h change and total balance over all wallets for each
	 *		user per mint.
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
		WHERE 1=1
			` + mintFilter + `
			` + ownerIdFilter + `
			` + tickerFilter + `
		ORDER BY artist_coins.created_at ASC
		LIMIT @limit
		OFFSET @offset
	`

	rows, err := app.pool.Query(c.Context(), sql, pgx.NamedArgs{
		"tickers":   queryParams.Tickers,
		"mints":     queryParams.Mints,
		"owner_ids": queryParams.OwnerIds,
		"limit":     queryParams.Limit,
		"offset":    queryParams.Offset,
	})
	if err != nil {
		return err
	}

	coinRows, err := pgx.CollectRows(rows, pgx.RowToStructByName[ArtistCoin])
	if err != nil {
		return err
	}

	wg := sync.WaitGroup{}
	for i := range coinRows {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			mint := coinRows[i].Mint
			overview, err := app.birdeyeClient.GetTokenOverview(c.Context(), mint, "24h")
			if err != nil {
				app.logger.Error("Error fetching token overview",
					zap.String("mint", mint),
					zap.Error(err))
				return
			}
			coinRows[i].TokenInfo = overview
		}(i)
	}
	wg.Wait()

	return c.JSON(fiber.Map{
		"data": coinRows,
	})
}
