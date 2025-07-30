package api

import (
	"time"

	"bridgerton.audius.co/trashid"
	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"
)

type GetArtistCoinsQueryParams struct {
	Tickers  []string         `query:"ticker"`
	Mints    []string         `query:"mint"`
	OwnerIds []trashid.HashId `query:"owner_id"`
	Limit    int              `query:"limit" default:"50" validate:"min=1,max=100"`
	Offset   int              `query:"offset" default:"0" validate:"min=0"`
}

type ArtistCoin struct {
	Ticker    string         `json:"ticker"`
	Mint      string         `json:"mint"`
	Decimals  int            `json:"decimals"`
	OwnerId   trashid.HashId `db:"user_id" json:"owner_id"`
	CreatedAt time.Time      `json:"created_at"`
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
		SELECT 
			artist_coins.ticker,
			artist_coins.mint,
			artist_coins.decimals,
			artist_coins.user_id,
			artist_coins.created_at
		FROM artist_coins
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

	return c.JSON(fiber.Map{
		"data": coinRows,
	})
}
