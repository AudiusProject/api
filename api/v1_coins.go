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

type ArtistCoin struct {
	Ticker                  string                 `json:"ticker"`
	Mint                    string                 `json:"mint"`
	UserId                  trashid.HashId         `json:"user_id"`
	Members                 int                    `json:"members"`
	Members24hChangePercent *float64               `json:"members_24h_change_percent"`
	CreatedAt               time.Time              `json:"created_at"`
	TokenInfo               *birdeye.TokenOverview `db:"-" json:"token_info,omitempty"`
}

func (app *ApiServer) v1Coins(c *fiber.Ctx) error {
	mintQueryParams := queryMulti(c, "mint")
	artistIdQueryParams := queryMulti(c, "artist_id")

	mintFilter := ""
	if len(mintQueryParams) > 0 {
		mintFilter = `AND mint = ANY(@mints)`
	}
	artistIdFilter := ""
	if len(artistIdQueryParams) > 0 {
		artistIdFilter = `AND user_id = ANY(@artist_ids)`
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
		WHERE 1=1
			` + mintFilter + `
			` + artistIdFilter + `
	`

	rows, err := app.pool.Query(c.Context(), sql, pgx.NamedArgs{
		"mints":      mintQueryParams,
		"artist_ids": artistIdQueryParams,
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
