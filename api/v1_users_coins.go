package api

import (
	"fmt"
	"strings"

	"bridgerton.audius.co/trashid"
	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"
)

type GetUsersCoinsQueryParams struct {
	Limit  int `query:"limit" default:"50" validate:"min=1,max=100"`
	Offset int `query:"offset" default:"0" validate:"min=0"`
}

type UserCoin struct {
	Ticker     string         `json:"ticker"`
	Mint       string         `json:"mint"`
	Decimals   int            `json:"decimals"`
	OwnerID    trashid.HashId `json:"owner_id"`
	Balance    float64        `json:"balance"`
	BalanceUSD float64        `json:"balance_usd"`
}

func (app *ApiServer) v1UsersCoins(c *fiber.Ctx) error {
	queryParams := GetUsersCoinsQueryParams{}
	if err := app.ParseAndValidateQueryParams(c, &queryParams); err != nil {
		return err
	}

	mintSql := `
		SELECT mint
		FROM artist_coins;
	`
	var mints []string
	rows, err := app.pool.Query(c.Context(), mintSql)
	if err != nil {
		return fmt.Errorf("failed to query mints: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var mint string
		if err := rows.Scan(&mint); err != nil {
			return fmt.Errorf("failed to scan mint: %w", err)
		}
		mints = append(mints, mint)
	}

	prices, err := app.birdeyeClient.GetPrices(c.Context(), mints)
	if err != nil || prices == nil {
		return fmt.Errorf("failed to get prices: %w", err)
	}

	pricesRows := make([]string, 0, len(*prices))
	for mint, price := range *prices {
		pricesRows = append(pricesRows, fmt.Sprintf("('%s', %f)", mint, price.Value))
	}

	sql := `
		WITH prices (mint, price) AS (
			VALUES ` + strings.Join(pricesRows, ",\n\t\t\t") + `
		), balances AS (
			SELECT
				user_bank_balances.mint,
				user_bank_balances.balance AS balance,
				user_bank_balances.account AS account,
				user_bank_balances.owner AS owner,
				TRUE as is_in_app_wallet
			FROM users
			JOIN sol_claimable_accounts 
				ON sol_claimable_accounts.ethereum_address = users.wallet
			JOIN sol_token_account_balances AS user_bank_balances 
				ON user_bank_balances.account = sol_claimable_accounts.account
			WHERE users.user_id = @user_id
			UNION ALL 
			SELECT
				associated_wallet_balances.mint,
				associated_wallet_balances.balance AS balance,
				associated_wallet_balances.account AS account,
				associated_wallet_balances.owner AS owner,
				FALSE as is_in_app_wallet
			FROM users
			JOIN associated_wallets 
				ON associated_wallets.user_id = users.user_id
			JOIN sol_token_account_balances AS associated_wallet_balances 
				ON associated_wallet_balances.owner = associated_wallets.wallet 
				AND associated_wallets.chain = 'sol'
			WHERE associated_wallets.user_id = @user_id
		), balances_by_mint AS (
			SELECT
				balances.mint,
				SUM(balances.balance) AS balance
			FROM balances
			GROUP BY balances.mint
		),
		balances_with_prices AS (
			SELECT 
				artist_coins.ticker,
				balances_by_mint.mint,
				artist_coins.decimals,
				artist_coins.user_id,
				balances_by_mint.balance AS balance,
				(balances_by_mint.balance * prices.price) / POWER(10, artist_coins.decimals) AS balance_usd
			FROM balances_by_mint
			JOIN prices ON balances_by_mint.mint = prices.mint
			JOIN artist_coins ON artist_coins.mint = balances_by_mint.mint
		)
		SELECT
			balances_with_prices.ticker,
			balances_with_prices.mint,
			balances_with_prices.decimals,
			balances_with_prices.user_id AS owner_id,
			balances_with_prices.balance,
			balances_with_prices.balance_usd
		FROM balances_with_prices
		ORDER BY
			balances_with_prices.ticker = '$AUDIO' DESC,
			balances_with_prices.balance_usd DESC,
			balances_with_prices.mint ASC
		LIMIT @limit
		OFFSET @offset
	;`

	rows, err = app.pool.Query(c.Context(), sql, pgx.NamedArgs{
		"user_id": app.getUserId(c),
		"limit":   queryParams.Limit,
		"offset":  queryParams.Offset,
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
