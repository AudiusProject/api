package api

import (
	"fmt"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"
)

type GetUsersCoinRouteParams struct {
	Mint string `params:"mint"`
}

func (app *ApiServer) v1UsersCoin(c *fiber.Ctx) error {
	params := GetUsersCoinRouteParams{}
	if err := c.ParamsParser(&params); err != nil {
		return err
	}
	prices, err := app.birdeyeClient.GetPrices(c.Context(), []string{params.Mint})
	if err != nil || prices == nil {
		return fmt.Errorf("failed to get price: %w", err)
	}

	sql := `
		WITH balances AS (
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
				AND user_bank_balances.mint = @mint
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
				AND associated_wallet_balances.mint = @mint
		), balances_by_mint AS (
			SELECT
				balances.mint,
				SUM(balances.balance) AS balance
			FROM balances
			GROUP BY balances.mint
		)
		SELECT 
			artist_coins.ticker,
			balances_by_mint.mint,
			balances_by_mint.balance AS balance,
			(balances_by_mint.balance * @price) / POWER(10, artist_coins.decimals) AS balance_usd
		FROM balances_by_mint
		JOIN artist_coins ON artist_coins.mint = balances_by_mint.mint
	;`

	rows, err := app.pool.Query(c.Context(), sql, pgx.NamedArgs{
		"user_id": app.getUserId(c),
		"mint":    params.Mint,
		"price":   (*prices)[params.Mint].Value,
	})
	if err != nil {
		return err
	}

	userCoin, err := pgx.CollectExactlyOneRow(rows, pgx.RowToStructByName[UserCoin])
	if err != nil {
		return err
	}

	return c.JSON(fiber.Map{
		"data": userCoin,
	})
}
