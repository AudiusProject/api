package api

import (
	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"
)

type GetWalletCoinsQueryParams struct {
	Limit  int `query:"limit" default:"50" validate:"min=1,max=100"`
	Offset int `query:"offset" default:"0" validate:"min=0"`
}

type WalletCoinsRouteParams struct {
	WalletId string `params:"walletId"`
}

func (app *ApiServer) v1WalletCoins(c *fiber.Ctx) error {
	params := WalletCoinsRouteParams{}
	if err := c.ParamsParser(&params); err != nil {
		return err
	}

	queryParams := GetWalletCoinsQueryParams{}
	if err := app.ParseAndValidateQueryParams(c, &queryParams); err != nil {
		return err
	}

	sql := `
		WITH balances_by_mint AS (
			SELECT
				balances.mint,
				SUM(balances.balance) AS balance
			FROM sol_token_account_balances AS balances
			WHERE balances.owner = @wallet_address
			GROUP BY balances.mint
		)
		SELECT
			artist_coins.ticker,
			artist_coins.mint,
			artist_coins.decimals,
			artist_coins.has_discord,
			artist_coins.user_id AS owner_id,
			COALESCE(balances_by_mint.balance, 0) AS balance,
			(COALESCE(balances_by_mint.balance, 0) * COALESCE(stats.price, pools.price_usd)) / POWER(10, artist_coins.decimals) AS balance_usd
		FROM artist_coins
		LEFT JOIN balances_by_mint ON balances_by_mint.mint = artist_coins.mint
		LEFT JOIN artist_coin_stats stats ON stats.mint = artist_coins.mint
		LEFT JOIN artist_coin_pools pools ON pools.base_mint = artist_coins.mint
		WHERE balance > 0  -- Show coins with positive balance
		ORDER BY
			-- Prioritize AUDIO
			artist_coins.ticker = 'AUDIO' DESC,
			-- Then by number of coins (balance)
			balance DESC,
			-- Finally by mint for consistent ordering
			artist_coins.mint ASC
		LIMIT @limit
		OFFSET @offset
	;`

	rows, err := app.pool.Query(c.Context(), sql, pgx.NamedArgs{
		"wallet_address": params.WalletId,
		"limit":          queryParams.Limit,
		"offset":         queryParams.Offset,
	})
	if err != nil {
		return err
	}

	userCoins, err := pgx.CollectRows(rows, pgx.RowToStructByName[UserCoin])
	if err != nil {
		return err
	}

	return c.JSON(fiber.Map{
		"data": userCoins,
	})
}
