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

	sql := `
		WITH member_counts_24h_ago AS (
			SELECT 
				COUNT(DISTINCT balances_24h_ago.user_id) AS members,
				balances_24h_ago.mint
			FROM (
				SELECT DISTINCT ON (
						sol_token_account_balance_changes.mint, 
						sol_token_account_balance_changes.account, 
						users.user_id, 
						associated_wallets.user_id
					)
					sol_token_account_balance_changes.mint,
					sol_token_account_balance_changes.account,
					sol_token_account_balance_changes.balance,
					COALESCE(associated_wallets.user_id, users.user_id) AS user_id
				FROM sol_token_account_balance_changes
				LEFT JOIN associated_wallets 
					ON associated_wallets.wallet = sol_token_account_balance_changes.owner
				LEFT JOIN sol_claimable_accounts 
					ON sol_claimable_accounts.account = sol_token_account_balance_changes.account
				LEFT JOIN users ON users.wallet = sol_claimable_accounts.ethereum_address
				WHERE block_timestamp < NOW() - INTERVAL '24 hours'
					AND (associated_wallets.user_id IS NOT NULL OR users.user_id IS NOT NULL)
				ORDER BY 
					sol_token_account_balance_changes.mint, 
					sol_token_account_balance_changes.account, 
					users.user_id, 
					associated_wallets.user_id, 
					block_timestamp DESC
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
				LEFT JOIN associated_wallets 
					ON associated_wallets.wallet = sol_token_account_balances.owner
				LEFT JOIN sol_claimable_accounts 
					ON sol_claimable_accounts.account = sol_token_account_balances.account
				LEFT JOIN users 
					ON users.wallet = sol_claimable_accounts.ethereum_address
				WHERE sol_token_account_balances.balance > 0
					AND (associated_wallets.wallet IS NOT NULL 
							OR sol_claimable_accounts.account IS NOT NULL)
			) AS member_balances
			GROUP BY member_balances.mint
		)
		SELECT 
			artist_coins.ticker,
			artist_coins.mint,
			artist_coins.user_id,
			artist_coins.created_at,
			COALESCE(member_counts.members, 0) AS members,
			(
				(COALESCE(member_counts.members, 0) - member_counts_24h_ago.members) * 100.0 /
				NULLIF(member_counts_24h_ago.members, 0)
			) AS members_24h_change_percent
		FROM artist_coins
		LEFT JOIN member_counts ON artist_coins.mint = member_counts.mint
		LEFT JOIN member_counts_24h_ago ON artist_coins.mint = member_counts_24h_ago.mint
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
